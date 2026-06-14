package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

var (
	fcmClient     *messaging.Client
	fcmClientOnce sync.Once
	fcmInitErr    error
)

// InitFirebase initializes the Firebase Admin SDK.
// Call this once during application startup.
func InitFirebase() error {
	fcmClientOnce.Do(func() {
		ctx := context.Background()
		var app *firebase.App

		// 1. Check for Base64 encoded credentials (often used in Railway env vars)
		if credsBase64 := os.Getenv("FIREBASE_CREDENTIALS_BASE64"); credsBase64 != "" {
			credsBase64 = strings.TrimRight(credsBase64, "% \r\n")
			decoded, err := base64.StdEncoding.DecodeString(credsBase64)
			if err != nil {
				log.Printf("Warning: Failed to decode FIREBASE_CREDENTIALS_BASE64: %v", err)
			} else {
				opt := option.WithCredentialsJSON(decoded)
				app, fcmInitErr = firebase.NewApp(ctx, nil, opt)
				if fcmInitErr == nil {
					log.Println("Firebase initialized using FIREBASE_CREDENTIALS_BASE64")
				}
			}
		}

		// 2. Fallback to raw JSON credentials
		if app == nil && fcmInitErr == nil {
			if credsJSON := os.Getenv("FIREBASE_CREDENTIALS_JSON"); credsJSON != "" {
				opt := option.WithCredentialsJSON([]byte(credsJSON))
				app, fcmInitErr = firebase.NewApp(ctx, nil, opt)
				if fcmInitErr == nil {
					log.Println("Firebase initialized using FIREBASE_CREDENTIALS_JSON")
				}
			}
		}

		// 3. Fallback to path config
		if app == nil && fcmInitErr == nil {
			if credPath := os.Getenv("FIREBASE_CREDENTIALS_PATH"); credPath != "" {
				opt := option.WithCredentialsFile(credPath)
				app, fcmInitErr = firebase.NewApp(ctx, nil, opt)
				if fcmInitErr == nil {
					log.Println("Firebase initialized using FIREBASE_CREDENTIALS_PATH")
				}
			}
		}

		// 4. Default Credentials fallback
		if app == nil && fcmInitErr == nil {
			app, fcmInitErr = firebase.NewApp(ctx, nil)
		}

		if fcmInitErr != nil {
			log.Printf("Warning: Firebase initialization failed: %v", fcmInitErr)
			log.Println("FCM notifications will be disabled. Set FIREBASE_CREDENTIALS_PATH or GOOGLE_APPLICATION_CREDENTIALS.")
			return
		}

		fcmClient, fcmInitErr = app.Messaging(ctx)
		if fcmInitErr != nil {
			log.Printf("Warning: Firebase Messaging client creation failed: %v", fcmInitErr)
			return
		}

		log.Println("Firebase Admin SDK initialized successfully")
	})

	return fcmInitErr
}

// SendHarvestNotification sends a push notification to all subscribed devices
// when a batch has completed fermentation and is ready for harvest.
func SendHarvestNotification(batchName string) {
	if fcmClient == nil {
		log.Printf("FCM: Client not initialized. Skipping notification for batch '%s'", batchName)
		fmt.Printf("SIMULASI NOTIFIKASI: Tempe Siap Panen! - Batch %s siap dipanen.\n", batchName)
		return
	}

	ctx := context.Background()

	message := &messaging.Message{
		Topic: "tempura_harvest",
		Notification: &messaging.Notification{
			Title: "🎉 Tempe Siap Panen!",
			Body:  fmt.Sprintf("Batch %s siap dipanen.", batchName),
		},
		Data: map[string]string{
			"type":       "harvest",
			"batch_name": batchName,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Sound:    "default",
				Priority: messaging.PriorityHigh,
			},
		},
	}

	response, err := fcmClient.Send(ctx, message)
	if err != nil {
		log.Printf("FCM: Failed to send harvest notification: %v", err)
		return
	}

	log.Printf("FCM: Harvest notification sent successfully. Response: %s", response)
}
