<!-- Code generated by gomarkdoc. DO NOT EDIT -->

# softserve

```go
import "github.com/packetdancer/softserve/v1"
```

Package softserve implements a very basic concurrent webserver in Go, suitable for creating simple webservices. While the bulk of the functionality is provided by the golang net/http package, softserve basically provides all the framework glue to load a configuration file, set up http or https instances, handle serving directories as\-desired, and so on.

This is not code meant for high\-volume webservices; it was originally created to facilitate automation off of github commit hooks, but is quite suitable for such tasks. To use it at higher volume, better concurrency would be required in a few areas.

It was created by Rachel Blackman for personal use, but is made available freely for whoever wants to use it. Attribution is nice, but not required; feel free to use, modify, redistribute, print out and turn into origami, translate into Perl, or otherwise abuse this code.

NOTE: This version of softserve has had the packaging data stripped out, so that it can be used as a subpackage for the "Gatun" demo; if you want to use it in a different project, pull from the actual package out on the internet.

## Index

- [func GetContentType(filePath string) (string, error)](<#func-getcontenttype>)
- [func ReadRequestBody(req *http.Request) ([]byte, error)](<#func-readrequestbody>)
- [type DiskRecord](<#type-diskrecord>)
- [type Redirect](<#type-redirect>)
- [type Server](<#type-server>)
  - [func (s *Server) Configure(conf ServerConfig) error](<#func-server-configure>)
  - [func (s *Server) Finalize() error](<#func-server-finalize>)
  - [func (s *Server) IsRunning() bool](<#func-server-isrunning>)
  - [func (s *Server) RegisterHandler(path string, handler http.HandlerFunc) error](<#func-server-registerhandler>)
  - [func (s *Server) Start() error](<#func-server-start>)
  - [func (s *Server) Stop(blocking bool) error](<#func-server-stop>)
  - [func (s *Server) Wait() error](<#func-server-wait>)
- [type ServerConfig](<#type-serverconfig>)
  - [func (sc *ServerConfig) Initialize()](<#func-serverconfig-initialize>)
  - [func (sc *ServerConfig) ReadConfigYAML(configFile string) error](<#func-serverconfig-readconfigyaml>)
  - [func (sc *ServerConfig) Validate() error](<#func-serverconfig-validate>)


## func GetContentType

```go
func GetContentType(filePath string) (string, error)
```

GetContentType takes a file and determines what MIME type it is. This detection is performed by examining the first 512 bytes of the file \(or less, if the file is shorter\) and using the same sort of "magic" system that Apache uses, as implemented by the default golang http.DetectContentType function.

## func ReadRequestBody

```go
func ReadRequestBody(req *http.Request) ([]byte, error)
```

ReadRequestBody is a helper function which, given a request from the http/https engine, will extract the body of the request. This is generally most useful for POST operations. Decoding the actual raw byte array is up to the calling code, however.

## type DiskRecord

DiskRecord contains a mapping of an on\-disk directory or file to a web location within our server. It is exported simply so that it can be used in a ServerConfig.

```go
type DiskRecord struct {
    // WebPath is the location within our website we should serve this directory on.
    WebPath string `yaml:"web-path"`

    // FilePath is the path on disk to serve.
    FilePath string `yaml:"file-path"`

    // An optional content-type, if it's necessary to manually override what might be auto-detected.
    ContentType string `yaml:"content-type"`
}
```

## type Redirect

Redirect contains a redirection mapping, marking that a given web path should be served as a redirection notice to a different one. It is exported simply so that it can be used in a ServerConfig.

```go
type Redirect struct {
    // OldPath is the path we should serve a redirect on.
    OldPath string `yaml:"path"`

    // NewPath is what we should redirect to.
    NewPath string `yaml:"new-path"`

    // Code is the HTTP status code in the 3xx range determining the redirect type (permanent, temporary, etc.)
    Code int `yaml:"code"`
}
```

## type Server

Server is the structure which holds all the state for a given SoftServe instance \-\- all the content providers, configuration, and current run state. This structure should be treated as opaque; none of the fields are exported, because none should be directly modified.

```go
type Server struct {
    // contains filtered or unexported fields
}
```

### func \(\*Server\) Configure

```go
func (s *Server) Configure(conf ServerConfig) error
```

Configure attempts to set up a server using the provided ServerConfig. This function must only ever be called when the server is not presently running.

### func \(\*Server\) Finalize

```go
func (s *Server) Finalize() error
```

Finalize will lock in a server's configuration, setting up all the content providers. Once a server has been Finalized, you cannot reconfigure it, and a server must be Finalized before you can start it running. If Start is called before Finalize, it will call Finalize as part of its initialization.

### func \(\*Server\) IsRunning

```go
func (s *Server) IsRunning() bool
```

IsRunning simply returns whether or not the server is currently running.

### func \(\*Server\) RegisterHandler

```go
func (s *Server) RegisterHandler(path string, handler http.HandlerFunc) error
```

RegisterHandler takes a given http.HandlerFunc and sets it up to handle the provided path. It's worth noting the path is a pattern, so you can have it dynamically handle pieces underneath it; e.g., /foo can also handle /foo/bar if nothing else does.

### func \(\*Server\) Start

```go
func (s *Server) Start() error
```

Start will set a SoftServe instance running, and begin serving pages on the appropriate ports. If Finalize has not been called before Start, it will be implicitly called as part of startup.

### func \(\*Server\) Stop

```go
func (s *Server) Stop(blocking bool) error
```

Stop will shut down a SoftServe instance. If the blocking parameter is true, it will not return from this call until the server has stopped; otherwise, it will return immediately and perform the shutdown in the background.

### func \(\*Server\) Wait

```go
func (s *Server) Wait() error
```

Wait will block until the running server has stopped. If the server is not running, it will return immediately with an appropriate error message.

## type ServerConfig

ServerConfig encapsulates the configuration of a SoftServe server, suitable for loading from or saving to a yaml or XML file. While there is a helper function to load from a YAML file, you can also embed the ServerConfig struct in your own more expansive configuration, and it will parse properly as a child element.

```go
type ServerConfig struct {

    // Basic contains the configuration for the insecure HTTP server, if used.
    Basic struct {

        // Enabled determines whether we create a http server or not.
        Enabled bool `yaml:"enabled"`

        // Port is the port which we will attempt to bind to for a http server.
        Port int16 `yaml:"port"`
    }   `yaml:"http" xml:"Server"`

    // Secure contains the configuration for the secured HTTPS server, if used.
    Secure struct {

        // Enabled determines whether we create an https server or not.
        Enabled bool `yaml:"enabled"`

        // Port is the port which we will attempt to bind to for a https server.
        Port int16 `yaml:"port"`

        // CertificateFile is the on-disk path to the certificate we will use.
        CertificateFile string `yaml:"certificate"`

        // KeyFile is the on-disk path to the key for the CertificateFile.
        KeyFile string `yaml:"key""`

        // CACertFile is the on-disk path to the certificate authority who signed CertificateFile.
        CACertFile string `yaml:"authority"`
    }   `yaml:"https" xml:"SecureServer"`

    // DocumentRoot is where we should serve files from, by default.
    DocumentRoot string `yaml:"document_root"`

    // Directories contains additional directories we should serve files from.
    Directories []DiskRecord `yaml:"directories" xml:">Directory"`

    // Files contains specific files we should serve at specific locations.
    Files []DiskRecord `yaml:"files" xml:">File"`

    // Redirects contains any redirects we want to add to our server.
    Redirects []Redirect `yaml:"redirects" xml:">Redirect"`
}
```

### func \(\*ServerConfig\) Initialize

```go
func (sc *ServerConfig) Initialize()
```

Initialize resets a ServerConfig to the default values.

### func \(\*ServerConfig\) ReadConfigYAML

```go
func (sc *ServerConfig) ReadConfigYAML(configFile string) error
```

ReadConfigYAML does what you'd expect; it reads a configuration from a provided YAML file.

### func \(\*ServerConfig\) Validate

```go
func (sc *ServerConfig) Validate() error
```

Validate attempts to validate a given ServerConfiguration, returning a descriptive error if anything is wrong.



Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)
