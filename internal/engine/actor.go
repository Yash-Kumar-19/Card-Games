package engine

import (
	"fmt"
	"log"
	"time"

	"github.com/nakad/cardgames/internal/game"
)

// TableEvent is a message sent to a table actor.
type TableEvent struct {
	Type     string
	PlayerID string
	Action   *game.Action
	Reply    chan TableReply
}

// TableReply is the response from the table actor.
type TableReply struct {
	Err       error
	Broadcast []BroadcastMsg
}

// BroadcastMsg is a message to be sent to one or all players.
type BroadcastMsg struct {
	TargetID string // empty = broadcast to all at table
	Type     string
	Payload  any
}

// TableActor manages a table's lifecycle in a single goroutine.
type TableActor struct {
	Table       *Table
	Events      chan TableEvent
	done        chan struct{}
	turnTimer   *time.Timer
	TurnTimeout time.Duration
	OnBroadcast func(tableID string, msgs []BroadcastMsg)
}

// NewTableActor creates and starts a new table actor.
func NewTableActor(table *Table, turnTimeout time.Duration) *TableActor {
	a := &TableActor{
		Table:       table,
		Events:      make(chan TableEvent, 64),
		done:        make(chan struct{}),
		TurnTimeout: turnTimeout,
	}
	go a.run()
	return a
}

func (a *TableActor) run() {
	defer close(a.done)
	for {
		select {
		case evt, ok := <-a.Events:
			if !ok {
				return
			}
			a.handleEvent(evt)
		}
	}
}

func (a *TableActor) handleEvent(evt TableEvent) {
	reply := TableReply{}

	switch evt.Type {
	case "join":
		player := NewPlayer(evt.PlayerID, evt.PlayerID, 10000) // default balance
		err := a.Table.AddPlayer(player)
		if err != nil {
			reply.Err = err
		} else {
			reply.Broadcast = a.buildTableStateBroadcast("PLAYER_JOINED")
		}

	case "leave":
		err := a.Table.RemovePlayer(evt.PlayerID)
		if err != nil {
			reply.Err = err
		} else {
			reply.Broadcast = a.buildTableStateBroadcast("PLAYER_LEFT")
		}

	case "start":
		err := a.Table.StartRound()
		if err != nil {
			reply.Err = err
		} else {
			reply.Broadcast = a.buildDealBroadcast()
			a.resetTurnTimer()
		}

	case "action":
		if evt.Action == nil {
			reply.Err = fmt.Errorf("no action provided")
			break
		}
		err := a.Table.PlayerAction(evt.PlayerID, *evt.Action)
		if err != nil {
			reply.Err = err
			break
		}

		if a.Table.State == StateShowdown {
			a.stopTurnTimer()
			winners, err := a.Table.ResolveShowdown()
			if err != nil {
				reply.Err = err
			} else {
				reply.Broadcast = a.buildResultBroadcast(winners)
			}
		} else {
			reply.Broadcast = a.buildTurnBroadcast()
			a.resetTurnTimer()
		}

	case "timeout":
		// Auto-fold for the current player
		if a.Table.State != StateBetting || a.Table.GameState == nil {
			break
		}
		gs := a.Table.GameState
		if gs.CurrentTurn >= len(gs.ActivePlayers) {
			break
		}
		currentID := gs.ActivePlayers[gs.CurrentTurn]
		foldAction := game.Action{Type: game.ActionFold}
		err := a.Table.PlayerAction(currentID, foldAction)
		if err != nil {
			log.Printf("auto-fold error for %s: %v", currentID, err)
			break
		}

		if a.Table.State == StateShowdown {
			a.stopTurnTimer()
			winners, err := a.Table.ResolveShowdown()
			if err != nil {
				log.Printf("showdown error: %v", err)
			} else {
				reply.Broadcast = a.buildResultBroadcast(winners)
			}
		} else {
			reply.Broadcast = a.buildTurnBroadcast()
			a.resetTurnTimer()
		}

		// Broadcast directly since timeout has no reply channel
		if a.OnBroadcast != nil && len(reply.Broadcast) > 0 {
			a.OnBroadcast(a.Table.ID, reply.Broadcast)
		}
		return // no reply channel for timeout events
	}

	if evt.Reply != nil {
		evt.Reply <- reply
	}

	// Also broadcast via callback
	if a.OnBroadcast != nil && len(reply.Broadcast) > 0 {
		a.OnBroadcast(a.Table.ID, reply.Broadcast)
	}
}

func (a *TableActor) resetTurnTimer() {
	a.stopTurnTimer()
	if a.TurnTimeout <= 0 {
		return
	}
	a.turnTimer = time.AfterFunc(a.TurnTimeout, func() {
		a.Events <- TableEvent{Type: "timeout"}
	})
}

func (a *TableActor) stopTurnTimer() {
	if a.turnTimer != nil {
		a.turnTimer.Stop()
		a.turnTimer = nil
	}
}

// Stop shuts down the actor.
func (a *TableActor) Stop() {
	a.stopTurnTimer()
	close(a.Events)
	<-a.done
}

// Send sends an event to the actor and waits for the reply.
func (a *TableActor) Send(evt TableEvent) TableReply {
	evt.Reply = make(chan TableReply, 1)
	a.Events <- evt
	return <-evt.Reply
}

// --- broadcast builders ---

func (a *TableActor) buildTableStateBroadcast(eventType string) []BroadcastMsg {
	players := make([]map[string]any, len(a.Table.Players))
	for i, p := range a.Table.Players {
		players[i] = map[string]any{
			"id":      p.ID,
			"name":    p.Name,
			"balance": p.Balance,
		}
	}
	return []BroadcastMsg{{
		Type: eventType,
		Payload: map[string]any{
			"table_id": a.Table.ID,
			"state":    a.Table.State.String(),
			"players":  players,
		},
	}}
}

func (a *TableActor) buildDealBroadcast() []BroadcastMsg {
	msgs := []BroadcastMsg{}
	gs := a.Table.GameState

	// Send each player their own cards privately
	for _, p := range a.Table.Players {
		cards := gs.Hands[p.ID]
		cardDTOs := make([]map[string]string, len(cards))
		for i, c := range cards {
			cardDTOs[i] = map[string]string{
				"rank": c.Rank.String(),
				"suit": c.Suit.String(),
			}
		}
		msgs = append(msgs, BroadcastMsg{
			TargetID: p.ID,
			Type:     "DEAL_CARDS",
			Payload: map[string]any{
				"cards":       cardDTOs,
				"pot":         gs.Pot,
				"current_bet": gs.CurrentBet,
			},
		})
	}

	// Then send turn notification to all
	currentID := gs.ActivePlayers[gs.CurrentTurn]
	msgs = append(msgs, BroadcastMsg{
		Type: "TURN_CHANGE",
		Payload: map[string]any{
			"player_id":   currentID,
			"current_bet": gs.CurrentBet,
			"pot":         gs.Pot,
			"timeout_sec": int(a.TurnTimeout.Seconds()),
		},
	})

	return msgs
}

func (a *TableActor) buildTurnBroadcast() []BroadcastMsg {
	gs := a.Table.GameState
	if len(gs.ActivePlayers) == 0 {
		return nil
	}
	currentID := gs.ActivePlayers[gs.CurrentTurn]
	return []BroadcastMsg{{
		Type: "TURN_CHANGE",
		Payload: map[string]any{
			"player_id":   currentID,
			"current_bet": gs.CurrentBet,
			"pot":         gs.Pot,
			"timeout_sec": int(a.TurnTimeout.Seconds()),
		},
	}}
}

func (a *TableActor) buildResultBroadcast(winners []string) []BroadcastMsg {
	gs := a.Table.GameState

	// Reveal all hands
	hands := map[string][]map[string]string{}
	for pid, cards := range gs.Hands {
		cardDTOs := make([]map[string]string, len(cards))
		for i, c := range cards {
			cardDTOs[i] = map[string]string{
				"rank": c.Rank.String(),
				"suit": c.Suit.String(),
			}
		}
		hands[pid] = cardDTOs
	}

	winnerNames := make([]string, len(winners))
	for i, wid := range winners {
		for _, p := range a.Table.Players {
			if p.ID == wid {
				winnerNames[i] = p.Name
				break
			}
		}
	}

	// Balances after payout
	balances := map[string]int64{}
	for _, p := range a.Table.Players {
		balances[p.ID] = p.Balance
	}

	return []BroadcastMsg{{
		Type: "GAME_RESULT",
		Payload: map[string]any{
			"winners":  winners,
			"names":    winnerNames,
			"pot":      gs.Pot,
			"hands":    hands,
			"balances": balances,
		},
	}}
}
