package websocket

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

func TestWatchHubSuite(t *testing.T) {
	suite.Run(t, new(WatchHubSuite))
}

func (s *WatchHubSuite) TestSubscribeAndReceive() {
	t := s.T()
	projectId := "test-project"
	clientId := "test-client"

	// Create a channel to receive messages
	messages := make(chan []byte, 10)

	// Subscribe to the client
	sub, err := s.watchHub.Subscribe(projectId, clientId, messages)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	// Send a log line
	testMessage := []byte("test log line")
	err = s.watchHub.SendLogLine(projectId, clientId, testMessage)
	if err != nil {
		t.Fatalf("Failed to send log line: %v", err)
	}

	// Wait for the message
	select {
	case received := <-messages:
		if string(received) != string(testMessage) {
			t.Errorf("Expected message %q, got %q", testMessage, received)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func (s *WatchHubSuite) TestSubscribeToDifferentClients() {
	t := s.T()
	projectId := "test-project"
	client1 := "client-1"
	client2 := "client-2"

	// Create channels for each client
	messages1 := make(chan []byte, 10)
	messages2 := make(chan []byte, 10)

	// Subscribe to both clients
	sub1, err := s.watchHub.Subscribe(projectId, client1, messages1)
	if err != nil {
		t.Fatalf("Failed to subscribe to client1: %v", err)
	}
	defer sub1.Unsubscribe()

	sub2, err := s.watchHub.Subscribe(projectId, client2, messages2)
	if err != nil {
		t.Fatalf("Failed to subscribe to client2: %v", err)
	}
	defer sub2.Unsubscribe()

	// Send messages to each client
	msg1 := []byte("message for client 1")
	msg2 := []byte("message for client 2")

	err = s.watchHub.SendLogLine(projectId, client1, msg1)
	if err != nil {
		t.Fatalf("Failed to send to client1: %v", err)
	}

	err = s.watchHub.SendLogLine(projectId, client2, msg2)
	if err != nil {
		t.Fatalf("Failed to send to client2: %v", err)
	}

	// Verify client1 receives only its message
	select {
	case received := <-messages1:
		if string(received) != string(msg1) {
			t.Errorf("Client1 expected %q, got %q", msg1, received)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for client1 message")
	}

	// Verify client2 receives only its message
	select {
	case received := <-messages2:
		if string(received) != string(msg2) {
			t.Errorf("Client2 expected %q, got %q", msg2, received)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for client2 message")
	}

	// Verify no cross-talk (client1 shouldn't receive client2's message)
	select {
	case received := <-messages1:
		t.Errorf("Client1 received unexpected message: %q", received)
	case <-time.After(100 * time.Millisecond):
		// Expected - no additional messages
	}
}

func (s *WatchHubSuite) TestUnsubscribe() {
	t := s.T()
	projectId := "test-project"
	clientId := "unsubscribe-client"

	messages := make(chan []byte, 10)

	sub, err := s.watchHub.Subscribe(projectId, clientId, messages)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Send first message
	msg1 := []byte("first message")
	err = s.watchHub.SendLogLine(projectId, clientId, msg1)
	if err != nil {
		t.Fatalf("Failed to send first message: %v", err)
	}

	// Verify we receive it
	select {
	case received := <-messages:
		if string(received) != string(msg1) {
			t.Errorf("Expected %q, got %q", msg1, received)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for first message")
	}

	// Unsubscribe
	err = sub.Unsubscribe()
	if err != nil {
		t.Fatalf("Failed to unsubscribe: %v", err)
	}

	// Send second message
	msg2 := []byte("second message")
	err = s.watchHub.SendLogLine(projectId, clientId, msg2)
	if err != nil {
		t.Fatalf("Failed to send second message: %v", err)
	}

	// Should not receive the second message
	select {
	case received := <-messages:
		t.Errorf("Should not have received message after unsubscribe: %q", received)
	case <-time.After(100 * time.Millisecond):
		// Expected - no message after unsubscribe
	}
}

func (s *WatchHubSuite) TestMultipleSubscribersToSameClient() {
	t := s.T()
	projectId := "test-project"
	clientId := "shared-client"

	// Two subscribers to the same client
	messages1 := make(chan []byte, 10)
	messages2 := make(chan []byte, 10)

	sub1, err := s.watchHub.Subscribe(projectId, clientId, messages1)
	if err != nil {
		t.Fatalf("Failed to subscribe sub1: %v", err)
	}
	defer sub1.Unsubscribe()

	sub2, err := s.watchHub.Subscribe(projectId, clientId, messages2)
	if err != nil {
		t.Fatalf("Failed to subscribe sub2: %v", err)
	}
	defer sub2.Unsubscribe()

	// Send a message
	msg := []byte("broadcast message")
	err = s.watchHub.SendLogLine(projectId, clientId, msg)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Both subscribers should receive the message
	received1 := false
	received2 := false

	for i := 0; i < 2; i++ {
		select {
		case r := <-messages1:
			if string(r) != string(msg) {
				t.Errorf("Sub1 expected %q, got %q", msg, r)
			}
			received1 = true
		case r := <-messages2:
			if string(r) != string(msg) {
				t.Errorf("Sub2 expected %q, got %q", msg, r)
			}
			received2 = true
		case <-time.After(2 * time.Second):
			t.Fatalf("Timeout waiting for messages (received1=%v, received2=%v)", received1, received2)
		}
	}

	if !received1 || !received2 {
		t.Errorf("Not all subscribers received the message: received1=%v, received2=%v", received1, received2)
	}
}

func (s *WatchHubSuite) TestDifferentProjects() {
	t := s.T()
	project1 := "project-1"
	project2 := "project-2"
	clientId := "same-client-id"

	// Same client ID but different projects
	messages1 := make(chan []byte, 10)
	messages2 := make(chan []byte, 10)

	sub1, err := s.watchHub.Subscribe(project1, clientId, messages1)
	if err != nil {
		t.Fatalf("Failed to subscribe to project1: %v", err)
	}
	defer sub1.Unsubscribe()

	sub2, err := s.watchHub.Subscribe(project2, clientId, messages2)
	if err != nil {
		t.Fatalf("Failed to subscribe to project2: %v", err)
	}
	defer sub2.Unsubscribe()

	// Send to project1 only
	msg1 := []byte("project1 message")
	err = s.watchHub.SendLogLine(project1, clientId, msg1)
	if err != nil {
		t.Fatalf("Failed to send to project1: %v", err)
	}

	// Verify only project1 subscriber receives it
	select {
	case received := <-messages1:
		if string(received) != string(msg1) {
			t.Errorf("Project1 expected %q, got %q", msg1, received)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for project1 message")
	}

	// Project2 should not receive it
	select {
	case received := <-messages2:
		t.Errorf("Project2 should not have received message: %q", received)
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}
