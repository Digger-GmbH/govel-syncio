package main

import (
	"log"
	"net"
	"net/http"
	"os"

	"github.com/Digger-GmbH/govel-syncio/proto"
	"github.com/Digger-GmbH/govel-syncio/src/server"
	"github.com/Digger-GmbH/govel-syncio/src/syncio"
	"google.golang.org/grpc"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	storePath := env("CONNECTIONS_STORE_PATH", "data/connections.json")
	store, err := syncio.NewStore(storePath)
	if err != nil {
		log.Fatalf("connection store: %v", err)
	}
	if err := store.BootstrapFromJSON(os.Getenv("SYNCIO_CONNECTIONS_JSON")); err != nil {
		log.Fatalf("bootstrap connections: %v", err)
	}

	shopify := syncio.NewShopifyClient(env("SHOPIFY_HTTP_BASE", "http://127.0.0.1:8081"))
	engine := &syncio.Engine{Store: store, Shopify: shopify}
	grpcSrv := server.NewGRPCServer(store, engine)

	grpcAddr := env("GRPC_ADDR", ":50051")
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("listen gRPC: %v", err)
	}
	gs := grpc.NewServer()
	proto.RegisterSyncioServiceServer(gs, grpcSrv)

	go func() {
		log.Printf("gRPC listening on %s", grpcAddr)
		if err := gs.Serve(lis); err != nil {
			log.Fatalf("gRPC serve: %v", err)
		}
	}()

	httpSrv := server.NewHTTPServer(store, engine, shopify, "assets")
	httpAddr := env("HTTP_ADDR", ":8080")
	log.Printf("HTTP listening on %s (Admin SPA + JSON API); shopify=%s", httpAddr, shopify.BaseURL)
	if err := http.ListenAndServe(httpAddr, httpSrv.Mux); err != nil {
		log.Fatalf("HTTP serve: %v", err)
	}
}
