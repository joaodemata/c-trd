package common_test

import (
	"encoding/json"
	"testing"
	"time"

	cmm "c_trd/common"
)

// El frontend lee el evento en camelCase
func TestEventJSONIsCamelCase(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(cmm.Event{TypeEvent: cmm.EventOpportunitiesUpdated})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	want := `{"typeEvent":"OPPORTUNITIES-UPDATED","data":null}`
	if string(body) != want {
		t.Fatalf("got %s, want %s", body, want)
	}
}

// Publish en un hub nil (antes de asignar EventsHub) no hace nada
func TestPublishNilHubDoesNothing(t *testing.T) {
	t.Parallel()

	var hub *cmm.Hub
	hub.Publish(cmm.Event{TypeEvent: cmm.EventOpportunitiesUpdated})
}

// Publish nunca bloquea a quien publica, aunque el hub no este corriendo y la cola se llene
func TestPublishDoesNotBlockWhenQueueIsFull(t *testing.T) {
	t.Parallel()

	hub := cmm.NewHub()
	done := make(chan struct{})

	go func() {
		for i := 0; i < cap(hub.Broadcast)+10; i++ {
			hub.Publish(cmm.Event{TypeEvent: cmm.EventOpportunitiesUpdated})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish se bloqueo con la cola llena")
	}

	if len(hub.Broadcast) != cap(hub.Broadcast) {
		t.Fatalf("got %d eventos en cola, want %d", len(hub.Broadcast), cap(hub.Broadcast))
	}
}

// receiveEvents lee los eventos que llegan al cliente durante wait
func receiveEvents(client *cmm.Client, wait time.Duration) []cmm.Event {
	events := make([]cmm.Event, 0)
	timeout := time.After(wait)

	for {
		select {
		case event := <-client.Send:
			events = append(events, event)
		case <-timeout:
			return events
		}
	}
}

// Una rafaga del mismo tipo de evento llega como un solo evento
func TestRunCoalescesBurstIntoOneEvent(t *testing.T) {
	t.Parallel()

	hub := cmm.NewHub()
	hub.CoalesceWindow = 50 * time.Millisecond
	go hub.Run()

	client := &cmm.Client{Hub: hub, Send: make(chan cmm.Event, 32)}
	hub.Register <- client

	for i := 0; i < 20; i++ {
		hub.Publish(cmm.Event{TypeEvent: cmm.EventOpportunitiesUpdated, Data: i})
	}

	events := receiveEvents(client, 300*time.Millisecond)
	if len(events) != 1 {
		t.Fatalf("got %d eventos, want 1", len(events))
	}
	// Se envia el ultimo evento del tipo
	if events[0].Data != 19 {
		t.Fatalf("got data %v, want 19", events[0].Data)
	}
}

// Tipos de evento distintos en la misma ventana se envian una vez cada uno, en el orden en que llegaron
func TestRunSendsEachEventTypeOnce(t *testing.T) {
	t.Parallel()

	hub := cmm.NewHub()
	hub.CoalesceWindow = 50 * time.Millisecond
	go hub.Run()

	client := &cmm.Client{Hub: hub, Send: make(chan cmm.Event, 32)}
	hub.Register <- client

	hub.Publish(cmm.Event{TypeEvent: cmm.EventOpportunitiesUpdated})
	hub.Publish(cmm.Event{TypeEvent: "OTHER-EVENT"})
	hub.Publish(cmm.Event{TypeEvent: cmm.EventOpportunitiesUpdated})

	events := receiveEvents(client, 300*time.Millisecond)
	if len(events) != 2 {
		t.Fatalf("got %d eventos, want 2", len(events))
	}
	if events[0].TypeEvent != cmm.EventOpportunitiesUpdated || events[1].TypeEvent != "OTHER-EVENT" {
		t.Fatalf("got %s y %s, want %s y OTHER-EVENT", events[0].TypeEvent, events[1].TypeEvent, cmm.EventOpportunitiesUpdated)
	}
}

// Despues de cerrar una ventana, un evento nuevo abre otra y tambien se envia
func TestRunOpensNewWindowAfterFlush(t *testing.T) {
	t.Parallel()

	hub := cmm.NewHub()
	hub.CoalesceWindow = 50 * time.Millisecond
	go hub.Run()

	client := &cmm.Client{Hub: hub, Send: make(chan cmm.Event, 32)}
	hub.Register <- client

	hub.Publish(cmm.Event{TypeEvent: cmm.EventOpportunitiesUpdated})
	first := receiveEvents(client, 200*time.Millisecond)

	hub.Publish(cmm.Event{TypeEvent: cmm.EventOpportunitiesUpdated})
	second := receiveEvents(client, 200*time.Millisecond)

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("got %d y %d eventos, want 1 y 1", len(first), len(second))
	}
}

// Cada cliente registrado recibe el evento publicado
func TestPublishReachesRegisteredClients(t *testing.T) {
	t.Parallel()

	hub := cmm.NewHub()
	hub.CoalesceWindow = 10 * time.Millisecond
	go hub.Run()

	clients := []*cmm.Client{
		{Hub: hub, Send: make(chan cmm.Event, 1)},
		{Hub: hub, Send: make(chan cmm.Event, 1)},
	}
	for _, client := range clients {
		hub.Register <- client
	}

	hub.Publish(cmm.Event{TypeEvent: cmm.EventOpportunitiesUpdated})

	for i, client := range clients {
		select {
		case event := <-client.Send:
			if event.TypeEvent != cmm.EventOpportunitiesUpdated {
				t.Fatalf("cliente %d: got %s, want %s", i, event.TypeEvent, cmm.EventOpportunitiesUpdated)
			}
		case <-time.After(time.Second):
			t.Fatalf("cliente %d no recibio el evento", i)
		}
	}
}
