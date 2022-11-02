// Package softserve implements a very basic concurrent webserver in Go, suitable for creating simple
// webservices. While the bulk of the functionality is provided by the golang net/http package, softserve basically
// provides all the framework glue to load a configuration file, set up http or https instances, handle serving
// directories as-desired, and so on.
//
// This is not code meant for high-volume webservices; it was originally created to facilitate automation off of github
// commit hooks, but is quite suitable for such tasks. To use it at higher volume, better concurrency would be required
// in a few areas.
//
// It was created by Rachel Blackman for personal use, but is made available freely for whoever wants to use it.
// Attribution is nice, but not required; feel free to use, modify, redistribute, print out and turn into origami,
// translate into Perl, or otherwise abuse this code.
//
// NOTE: This version of softserve has had the packaging data stripped out, so that it can be used as a subpackage for
// the "Gatun" demo; if you want to use it in a different project, pull from the actual package out on the internet.
package softserve

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// Server is the structure which holds all the state for a given SoftServe instance -- all the content providers,
// configuration, and current run state. This structure should be treated as opaque; none of the fields are exported,
// because none should be directly modified.
type Server struct {

	// config holds the configuration for our server. This is not a pointer, or public, because we want it
	// only altered via the Configure function.
	config ServerConfig

	// running contains the run state of our server.
	running bool

	// finalized determines whether we can reconfigure the webserver. You can start and stop it repeatedly,
	// but cannot reconfigure it once finalized.
	finalized bool

	// handlers contains custom handlers we want to add when we start running the server
	handlers map[string]http.HandlerFunc

	// Our underlying net/http server implementation for our http server
	server *http.Server

	// Our underlying net/http server implementation for our https server
	secureServer *http.Server

	// Our waitGroup to keep the goroutines in sync
	waitGroup *sync.WaitGroup
}

// Configure attempts to set up a server using the provided ServerConfig. This function must only ever be called
// when the server is not presently running.
func (s *Server) Configure(conf ServerConfig) error {

	if s.running {
		return errors.New("server is running; you must configure it before running")
	}

	if s.finalized {
		return errors.New("server configuration has been finalized; too late to reconfigure now")
	}

	// Perform basic validation on our configuration before accepting it.
	err := conf.Validate()
	if err != nil {
		return err
	}

	s.config = conf
	s.server = nil
	s.secureServer = nil

	s.handlers = make(map[string]http.HandlerFunc, 0)

	return nil
}

// Finalize will lock in a server's configuration, setting up all the content providers. Once a server has been
// Finalized, you cannot reconfigure it, and a server must be Finalized before you can start it running. If Start
// is called before Finalize, it will call Finalize as part of its initialization.
func (s *Server) Finalize() error {

	if s.finalized {
		return errors.New("server configuration was already finalized")
	}

	if err := s.config.Validate(); err != nil {
		return errors.New(fmt.Sprintf("configuration error: %s", err.Error()))
	}

	s.finalized = true

	// Configure our handlers. First we go with any custom handlers...
	for path, handlerFunc := range s.handlers {
		http.Handle(path, handlerFunc)
	}

	// Then any redirects...
	for _, redirect := range s.config.Redirects {
		http.Handle(redirect.OldPath, http.RedirectHandler(redirect.NewPath, redirect.Code))
	}

	// Then any specific files...
	for _, record := range s.config.Files {
		handlerFunc, err := s.serveDocumentFunction(record.FilePath, record.ContentType)
		if err != nil {
			return errors.New(fmt.Sprintf("setup failure: %s", err.Error()))
		}
		http.HandleFunc(record.WebPath, handlerFunc)
	}

	// Then any directories...
	for _, record := range s.config.Directories {
		http.Handle(record.WebPath, http.FileServer(http.Dir(record.FilePath)))
	}

	// And lastly our fallback
	if len(s.config.DocumentRoot) > 0 {
		http.HandleFunc("/", s.serveDocumentRoot)
	}

	return nil

}

// IsRunning simply returns whether or not the server is currently running.
func (s *Server) IsRunning() bool {
	return s.running
}

// RegisterHandler takes a given http.HandlerFunc and sets it up to handle the provided path. It's worth noting
// the path is a pattern, so you can have it dynamically handle pieces underneath it; e.g., /foo can also handle
// /foo/bar if nothing else does.
func (s *Server) RegisterHandler(path string, handler http.HandlerFunc) error {

	if handler == nil {
		return errors.New("refusing to register a nil handler")
	}

	// Only do this if we're not running
	if s.running {
		return errors.New("server is already running; handlers can only be added when stopped")
	}

	// And not finalized
	if s.finalized {
		return errors.New("server configuration has been finalized; too late to reconfigure anything now")
	}

	// Check that our configuration does not in some way prevent this.
	{
		err := s.config.Validate()
		if err != nil {
			return errors.New(fmt.Sprintf("server is not correctly configured: %s", err.Error()))
		}

		if path == "/" && len(s.config.DocumentRoot) != 0 {
			return errors.New("cannot register a root handler with a DocumentRoot specified")
		}
	}

	// Check if our path already exists as a key in our handler table
	if _, ok := s.handlers[path]; ok {
		return errors.New("a handler already is registered at that path")
	}

	// Yay! Register it.
	s.handlers[path] = handler

	return nil
}

// serveDocumentFunction is the internal function which handles serving a specific file at a specific path.
func (s *Server) serveDocumentFunction(filePath string, contentType string) (http.HandlerFunc, error) {

	fileSize := 0

	// Check that this file exists.
	{
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("unable to serve document %s: %s", filePath, err.Error()))
		}

		if fileInfo.Size() == 0 {
			return nil, errors.New(fmt.Sprintf("unable to serve document %s: zero length file", filePath))
		}

		fileSize = int(fileInfo.Size())
	}

	if len(contentType) == 0 {
		ct, err := GetContentType(filePath)
		if err != nil {
			ct = "application/octet-stream"
		}

		contentType = ct
	}

	buffer := make([]byte, 0)

	{
		fp, err := os.Open(filePath)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("could not open file %s for serving: %s", filePath, err.Error()))
		}
		defer fp.Close()

		buffer = make([]byte, fileSize)
		_, err = fp.Read(buffer)
	}

	// Build a function that returns this static file.
	return func(response http.ResponseWriter, request *http.Request) {

		response.Header().Set("Content-Type", contentType)
		response.Header().Set("Content-Length", fmt.Sprintf("%d", fileSize))
		response.WriteHeader(200)
		response.Write(buffer)

	}, nil

}

// serveDocumentRoot is our baseline handler for the DocumentRoot, if provided.
func (s *Server) serveDocumentRoot(response http.ResponseWriter, request *http.Request) {

	path := filepath.Clean(request.URL.Path)
	if path[len(path)-1:] == "/" {
		path = path + "index.html"
	}

	filePath := filepath.Join(s.config.DocumentRoot, path)
	fileSize := 0

	// Check our file information.
	{
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			// If the file is not found, return a 404.
			if os.IsNotExist(err) {
				http.NotFound(response, request)
				return
			}

			// Generic 500 "internal error"
			http.Error(response, http.StatusText(500), 500)
			return
		}

		if fileInfo.IsDir() {
			// We're meant to be a very basic webserver for writing simple webservices,
			// not a replacement for Apache! Directory indexes is a bit beyond what we want.
			http.NotFound(response, request)
			return
		}

		// If it's a zero length file, treat it as not found.
		if fileInfo.Size() == 0 {
			http.NotFound(response, request)
			return
		}

		fileSize = int(fileInfo.Size())
	}

	contentType := "application/octet-stream"

	{
		fileType, err := GetContentType(filePath)
		if err != nil {
			// Generic "something went wrong" error.
			http.Error(response, http.StatusText(500), 500)
			return
		}

		contentType = fileType
	}

	buffer := make([]byte, fileSize)

	{
		fp, err := os.Open(filePath)
		if err != nil {
			http.Error(response, http.StatusText(500), 500)
			return
		}
		defer fp.Close()

		readSize, err2 := fp.Read(buffer)
		if err2 != nil || readSize != fileSize {
			http.Error(response, http.StatusText(500), 500)
			return
		}
	}

	response.Header().Set("Content-Type", contentType)
	response.Header().Set("Content-Length", fmt.Sprintf("%d", fileSize))
	response.WriteHeader(200)
	response.Write(buffer)

}

// Start will set a SoftServe instance running, and begin serving pages on the appropriate ports. If Finalize has not
// been called before Start, it will be implicitly called as part of startup.
func (s *Server) Start() error {

	if s.IsRunning() {
		return errors.New("server is already running")
	}

	// If our configuration hasn't already been locked in, do so.
	if !s.finalized {
		if err := s.Finalize(); err != nil {
			return err
		}
	}

	s.server = nil
	s.secureServer = nil
	s.waitGroup = &sync.WaitGroup{}

	// Configure our basic webserver, if we're going to use it.
	if s.config.Basic.Enabled {
		s.server = &http.Server{Addr: fmt.Sprintf(":%d", s.config.Basic.Port)}

		s.waitGroup.Add(1)

		// Launch a goroutine for our http server
		go func() {
			defer s.waitGroup.Done()

			if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
				log.Fatalf("fatal http server error: %s", err.Error())
			}
		}()
	}

	// Configure our secure webserver, if we're going to use it.
	if s.config.Secure.Enabled {
		s.secureServer = &http.Server{Addr: fmt.Sprintf(":%d", s.config.Secure.Port)}

		s.waitGroup.Add(1)

		// Launch a goroutine for our https server
		go func() {
			defer s.waitGroup.Done()

			if err := s.secureServer.ListenAndServeTLS(s.config.Secure.CertificateFile, s.config.Secure.KeyFile); err != http.ErrServerClosed {
				log.Fatalf("fatal https server error: %s", err.Error())
			}
		}()
	}

	// Lastly, set up a goroutine to watch our wait group.
	s.running = true
	go func() {
		s.waitGroup.Wait()

		// If both servers have exited, set our running to 'false'
		s.running = false
	}()

	return nil

}

// Stop will shut down a SoftServe instance. If the blocking parameter is true, it will not return from this call
// until the server has stopped; otherwise, it will return immediately and perform the shutdown in the background.
func (s *Server) Stop(blocking bool) error {

	if !s.IsRunning() {
		// Just exit. It's not worth giving an error that it wasn't running.
		return nil
	}

	if s.server != nil {
		// TODO: Maybe improve code to use real context? This is a super simple webservice framework, though...
		err := s.server.Shutdown(context.TODO())
		if err != nil {
			return err
		}
		s.server = nil
	}

	if s.secureServer != nil {
		// TODO: Maybe improve code to use real context? This is a super simple webservice framework, though...
		err := s.secureServer.Shutdown(context.TODO())
		if err != nil {
			return err
		}
		s.secureServer = nil
	}

	if blocking {
		s.waitGroup.Wait()
	}

	s.running = false

	return nil

}

// Wait will block until the running server has stopped. If the server is not running, it will return immediately
// with an appropriate error message.
func (s *Server) Wait() error {

	if !s.IsRunning() {
		return errors.New("server not running")
	}

	s.waitGroup.Wait()

	return nil
}
