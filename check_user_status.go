package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/minio/madmin-go/v3"
)

func main() {
	// Wait a bit for the user to be created
	time.Sleep(2 * time.Second)

	// Create admin client
	adminClient, err := madmin.New("localhost:9000", "minioadmin", "minioadmin", false)
	if err != nil {
		log.Fatal("Failed to create admin client:", err)
	}

	// Get user info
	userInfo, err := adminClient.GetUserInfo(context.Background(), "test-user")
	if err != nil {
		fmt.Printf("Error getting user info: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("User Status: '%s' (type: %T)\n", userInfo.Status, userInfo.Status)
	fmt.Printf("Expected: 'enabled'\n")
	fmt.Printf("Match: %t\n", userInfo.Status == "enabled")
	fmt.Printf("Full UserInfo: %+v\n", userInfo)
}
