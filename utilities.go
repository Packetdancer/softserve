package softserve

import (
	"bytes"
	"io"
	"net/http"
	"os"
)

// GetContentType takes a file and determines what MIME type it is. This detection is performed by examining the first
// 512 bytes of the file (or less, if the file is shorter) and using the same sort of "magic" system that Apache uses,
// as implemented by the default golang http.DetectContentType function.
func GetContentType(filePath string) (string, error) {

	// MIME detection only uses the first 512 bytes.
	buf := make([]byte, 512)

	fp, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer fp.Close()

	// Read those first 512 bytes. (This may look unsafe, but the Read function
	// checks the size of the slice before reading into it.)
	_, err = fp.Read(buf)
	if err != nil {
		return "", err
	}

	// Use the net/http library's MIME detection to get our content type.
	return http.DetectContentType(buf), nil
}

// ReadRequestBody is a helper function which, given a request from the http/https engine, will extract the body
// of the request. This is generally most useful for POST operations. Decoding the actual raw byte array is up to
// the calling code, however.
func ReadRequestBody(req *http.Request) ([]byte, error) {

	buffer := new(bytes.Buffer)
	_, err := io.Copy(buffer, req.Body)
	if err != nil {
		return make([]byte, 0), err
	}

	return buffer.Bytes(), nil

}
