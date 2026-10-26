package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	bucket := os.Getenv("MEDIA_BUCKET")
	if key == "" || bucket == "" {
		log.Fatal("set INFRAI_API_KEY and MEDIA_BUCKET")
	}
	client := infraiClient{key: key, http: &http.Client{Timeout: 40 * time.Second}}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	created, err := client.ensureBucket(ctx, bucket)
	if err != nil {
		log.Fatal(err)
	}
	if created {
		defer func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cleanupCancel()
			if err := client.deleteBucket(cleanupCtx, bucket); err != nil {
				log.Printf("delete bucket %q: %v", bucket, err)
			}
		}()
	}
	service := mediaService{client: client, bucket: bucket}
	server := &http.Server{Addr: "127.0.0.1:8080", Handler: http.HandlerFunc(service.serveHTTP)}
	stop, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	go func() {
		<-stop.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("serve: %v", err)
	}
}
