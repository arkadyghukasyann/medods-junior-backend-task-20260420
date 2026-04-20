package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"example.com/taskservice/internal/app"
	lambdaadapter "example.com/taskservice/internal/transport/lambda"
)

var (
	runtimeMu     sync.Mutex
	cachedHandler http.Handler
)

func main() {
	lambda.Start(handle)
}

func handle(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	handler, err := loadHandler(ctx)
	if err != nil {
		slog.New(slog.NewTextHandler(os.Stdout, nil)).Error("initialize runtime", "error", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			Body: `{"message":"internal server error"}`,
		}, nil
	}

	return lambdaadapter.Proxy(ctx, handler, event)
}

func loadHandler(ctx context.Context) (http.Handler, error) {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()

	if cachedHandler != nil {
		return cachedHandler, nil
	}

	runtimeInstance, err := app.NewRuntime(ctx, app.LoadConfig())
	if err != nil {
		return nil, err
	}

	cachedHandler = runtimeInstance.Router
	return cachedHandler, nil
}
