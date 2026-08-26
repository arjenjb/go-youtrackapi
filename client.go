package youtrackapi

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

type Client struct {
	baseUrl string
	client  *http.Client
	token   string
}

// Upload describes a file sent to a multipart YouTrack endpoint.
type Upload struct {
	Filename string
	Reader   io.Reader
}

func makeDecodedCall[T any](client *http.Client, req *http.Request, decode func(*JSONReader) (T, error)) (T, error) {
	var zero T
	resp, err := client.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	if err := checkResponse(resp); err != nil {
		return zero, err
	}
	return decode(NewJsonReader(resp.Body))
}

func makeDecodedListCall[T any](client *http.Client, req *http.Request, decode func(*JSONReader) (*T, error)) ([]T, error) {
	return makeDecodedCall(client, req, func(reader *JSONReader) ([]T, error) {
		return unmarshalList(reader, decode)
	})
}

func makeDecodedAbstractListCall[T any](client *http.Client, req *http.Request, decode func(*JSONReader) (T, error)) ([]T, error) {
	return makeDecodedCall(client, req, func(reader *JSONReader) ([]T, error) {
		return unmarshalAbstractList(reader, decode)
	})
}

func makeEmptyCall(client *http.Client, req *http.Request) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkResponse(resp)
}

func checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if len(data) == 0 {
		return fmt.Errorf("youtrack: unexpected HTTP status %s", resp.Status)
	}
	return fmt.Errorf("youtrack: unexpected HTTP status %s: %s", resp.Status, data)
}

func encodeMultipartFiles(files []Upload) ([]byte, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for i, file := range files {
		if file.Reader == nil {
			writer.Close()
			return nil, "", fmt.Errorf("upload %d has no reader", i)
		}
		part, err := writer.CreateFormFile(fmt.Sprintf("files[%d]", i), file.Filename)
		if err != nil {
			writer.Close()
			return nil, "", err
		}
		if _, err := io.Copy(part, file.Reader); err != nil {
			writer.Close()
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

func NewClient(baseURL string, token []byte) *Client {
	return &Client{baseUrl: baseURL, client: http.DefaultClient, token: string(token)}
}

// NewClientWithHTTPClient creates a client whose transport is responsible for
// authentication. This is used with oauth2.NewClient so expired access tokens
// are refreshed transparently.
func NewClientWithHTTPClient(baseURL string, client *http.Client) *Client {
	if client == nil {
		client = http.DefaultClient
	}

	return &Client{baseUrl: baseURL, client: client}
}
