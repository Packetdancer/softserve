package softserve

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"os"
)

// DiskRecord contains a mapping of an on-disk directory or file to a web location within our server. It is exported
// simply so that it can be used in a ServerConfig.
type DiskRecord struct {
	// WebPath is the location within our website we should serve this directory on.
	WebPath string `yaml:"web-path"`

	// FilePath is the path on disk to serve.
	FilePath string `yaml:"file-path"`

	// An optional content-type, if it's necessary to manually override what might be auto-detected.
	ContentType string `yaml:"content-type"`
}

// Redirect contains a redirection mapping, marking that a given web path should be served as a redirection notice
// to a different one. It is exported simply so that it can be used in a ServerConfig.
type Redirect struct {
	// OldPath is the path we should serve a redirect on.
	OldPath string `yaml:"path"`

	// NewPath is what we should redirect to.
	NewPath string `yaml:"new-path"`

	// Code is the HTTP status code in the 3xx range determining the redirect type (permanent, temporary, etc.)
	Code int `yaml:"code"`
}

// ServerConfig encapsulates the configuration of a SoftServe server, suitable for loading from or saving
// to a yaml or XML file. While there is a helper function to load from a YAML file, you can also embed the
// ServerConfig struct in your own more expansive configuration, and it will parse properly as a child element.
type ServerConfig struct {

	// Basic contains the configuration for the insecure HTTP server, if used.
	Basic struct {

		// Enabled determines whether we create a http server or not.
		Enabled bool `yaml:"enabled"`

		// Port is the port which we will attempt to bind to for a http server.
		Port int16 `yaml:"port"`
	} `yaml:"http" xml:"Server"`

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
	} `yaml:"https" xml:"SecureServer"`

	// DocumentRoot is where we should serve files from, by default.
	DocumentRoot string `yaml:"document_root"`

	// Directories contains additional directories we should serve files from.
	Directories []DiskRecord `yaml:"directories" xml:">Directory"`

	// Files contains specific files we should serve at specific locations.
	Files []DiskRecord `yaml:"files" xml:">File"`

	// Redirects contains any redirects we want to add to our server.
	Redirects []Redirect `yaml:"redirects" xml:">Redirect"`
}

// Initialize resets a ServerConfig to the default values.
func (sc *ServerConfig) Initialize() {

	sc.Basic.Enabled = false
	sc.Basic.Port = 80

	sc.Secure.Enabled = false
	sc.Secure.Port = 443
	sc.Secure.CertificateFile = ""
	sc.Secure.KeyFile = ""
	sc.Secure.CACertFile = ""

	sc.DocumentRoot = ""

	sc.Directories = make([]DiskRecord, 0)
	sc.Files = make([]DiskRecord, 0)
	sc.Redirects = make([]Redirect, 0)

}

// ReadConfigYAML does what you'd expect; it reads a configuration from a provided YAML file.
func (sc *ServerConfig) ReadConfigYAML(configFile string) error {

	// Clear our configuration.
	sc.Initialize()

	// Check if the file exists and is valid.
	s, err := os.Stat(configFile)
	if nil != err {
		return err
	}
	if s.IsDir() {
		return errors.New("file given was a directory")
	}

	// Open the file, with a deferred close if successful; this ensures the file
	// will be closed when the function completes.
	file, fileErr := os.OpenFile(configFile, os.O_RDONLY, 0644)
	if fileErr != nil {
		return fileErr
	}
	defer file.Close()

	// Parse the file.
	yamlDecode := yaml.NewDecoder(file)
	if yamlErr := yamlDecode.Decode(sc); yamlErr != nil {
		return yamlErr
	}

	return nil
}

// Validate attempts to validate a given ServerConfiguration, returning a descriptive error if anything is wrong.
func (sc *ServerConfig) Validate() error {

	// Check that we actually have a server of any type.
	if !sc.Basic.Enabled && !sc.Secure.Enabled {
		return errors.New("neither https or http servers are enabled; we have nothing to do")
	}

	// Check that https is correctly configured.
	if sc.Secure.Enabled && (len(sc.Secure.CertificateFile) == 0 || len(sc.Secure.KeyFile) == 0) {
		return errors.New("a certificate and key file must be provided if https is enabled")
	}

	if len(sc.DocumentRoot) != 0 {
		s, err := os.Stat(sc.DocumentRoot)
		if nil != err {
			return errors.New(fmt.Sprintf("invalid DocumentRoot %s: %s", sc.DocumentRoot, err.Error()))
		}

		if !s.IsDir() {
			return errors.New(fmt.Sprintf("invalid DocumentRoot %s: not a directory", sc.DocumentRoot))
		}
	}

	// Check that all our mapped directories are real.
	for _, directory := range sc.Directories {
		s, err := os.Stat(directory.FilePath)
		if nil != err {
			return errors.New(fmt.Sprintf("error reading served directory: %s", directory.FilePath))
		}
		if !s.IsDir() {
			return errors.New(fmt.Sprintf("attempted to serve file %s as a directory", directory.FilePath))
		}
	}

	for _, file := range sc.Files {
		s, err := os.Stat(file.FilePath)
		if nil != err {
			return errors.New(fmt.Sprintf("error reading served file: %s", file.FilePath))
		}
		if s.IsDir() {
			return errors.New(fmt.Sprintf("attempted to serve directory %s as a file", file.FilePath))
		}
	}

	for _, redirect := range sc.Redirects {
		if redirect.Code < 300 || redirect.Code > 399 {
			return errors.New(fmt.Sprintf("redirect %s has a non-redirection status code", redirect.OldPath))
		}
	}

	return nil
}
