package lambda

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

func Proxy(ctx context.Context, handler http.Handler, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	path := event.Path
	if path == "" {
		path = "/"
	}

	query := url.Values{}
	for key, values := range event.MultiValueQueryStringParameters {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	for key, value := range event.QueryStringParameters {
		if _, exists := query[key]; !exists {
			query.Add(key, value)
		}
	}

	bodyReader, err := requestBody(event)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			Body: `{"message":"invalid request body"}`,
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, event.HTTPMethod, path, bodyReader)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	req.URL.RawQuery = query.Encode()
	req.RequestURI = path
	if req.URL.RawQuery != "" {
		req.RequestURI += "?" + req.URL.RawQuery
	}

	for key, values := range event.MultiValueHeaders {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	for key, value := range event.Headers {
		if _, exists := req.Header[key]; !exists {
			req.Header.Set(key, value)
		}
	}

	if host := req.Header.Get("Host"); host != "" {
		req.Host = host
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	response := events.APIGatewayProxyResponse{
		StatusCode:        recorder.Code,
		Headers:           make(map[string]string, len(recorder.Header())),
		MultiValueHeaders: make(map[string][]string, len(recorder.Header())),
		Body:              recorder.Body.String(),
	}

	for key, values := range recorder.Header() {
		if len(values) == 0 {
			continue
		}

		response.Headers[key] = values[0]
		response.MultiValueHeaders[key] = append([]string(nil), values...)
	}

	return response, nil
}

func requestBody(event events.APIGatewayProxyRequest) (*bytes.Reader, error) {
	if event.Body == "" {
		return bytes.NewReader(nil), nil
	}

	if !event.IsBase64Encoded {
		return bytes.NewReader([]byte(event.Body)), nil
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(event.Body))
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(decoded), nil
}
