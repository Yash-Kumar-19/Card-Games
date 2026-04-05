package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/nakad/cardgames/internal/engine"
	"github.com/nakad/cardgames/internal/game"
	"github.com/nakad/cardgames/internal/games/teenpatti"
	tp "github.com/nakad/cardgames/internal/games/teenpatti"
	"github.com/nakad/cardgames/internal/model"
)

func main() {
	// Register game
	registry := game.NewRegistry()
	_ = registry.Register(teenpatti.New())

	g, _ := registry.Get("teen_patti")

	fmt.Println("=== Teen Patti CLI ===")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	// Create players
	p1 := model.NewPlayer("p1", "Alice", 1000)
	p2 := model.NewPlayer("p2", "Bob", 1000)
	p3 := model.NewPlayer("p3", "Charlie", 1000)

	table := engine.NewTable("table-1", g, 10)
	_ = table.AddPlayer(p1)
	_ = table.AddPlayer(p2)
	_ = table.AddPlayer(p3)

	for {
		fmt.Println("--- New Round ---")
		if err := table.StartRound(); err != nil {
			fmt.Printf("Error starting round: %v\n", err)
			return
		}

		fmt.Printf("Pot: %d | Boot: %d\n", table.GameState.Pot, table.BootAmount)
		fmt.Println()

		// Show each player their cards
		for _, p := range table.Players {
			hand := table.GameState.Hands[p.ID]
			rank := tp.EvaluateHand(hand)
			fmt.Printf("  %s's cards: %v (%s)\n", p.Name, hand, tp.CategoryName(rank.Category))
		}
		fmt.Println()

		// Betting loop
		for table.State == engine.StateBetting {
			gs := table.GameState
			currentID := gs.ActivePlayers[gs.CurrentTurn]
			var currentPlayer *model.Player
			for _, p := range table.Players {
				if p.ID == currentID {
					currentPlayer = p
					break
				}
			}

			actions := g.ValidActions(gs, currentID)
			fmt.Printf("[%s] Balance: %d | Pot: %d | Current Bet: %d\n",
				currentPlayer.Name, currentPlayer.Balance, gs.Pot, gs.CurrentBet)
			fmt.Printf("  Cards: %v (seen: %v)\n", gs.Hands[currentID], currentPlayer.IsSeen)
			fmt.Println("  Actions:")
			for i, a := range actions {
				if a.Amount > 0 {
					fmt.Printf("    %d) %s (cost: %d)\n", i+1, a.Type, a.Amount)
				} else {
					fmt.Printf("    %d) %s\n", i+1, a.Type)
				}
			}

			fmt.Print("  Choose action: ")
			scanner.Scan()
			input := strings.TrimSpace(scanner.Text())
			choice, err := strconv.Atoi(input)
			if err != nil || choice < 1 || choice > len(actions) {
				fmt.Println("  Invalid choice, try again.")
				continue
			}

			action := actions[choice-1]
			if err := table.PlayerAction(currentID, action); err != nil {
				fmt.Printf("  Error: %v\n", err)
				continue
			}

			fmt.Printf("  -> %s chose %s\n\n", currentPlayer.Name, action.Type)
		}

		// Showdown
		fmt.Println("--- Showdown ---")
		for _, pid := range table.GameState.ActivePlayers {
			hand := table.GameState.Hands[pid]
			rank := tp.EvaluateHand(hand)
			for _, p := range table.Players {
				if p.ID == pid {
					fmt.Printf("  %s: %v (%s)\n", p.Name, hand, tp.CategoryName(rank.Category))
					break
				}
			}
		}

		winners, err := table.ResolveShowdown()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("\nWinner(s): ")
		for _, wid := range winners {
			for _, p := range table.Players {
				if p.ID == wid {
					fmt.Printf("%s ", p.Name)
				}
			}
		}
		fmt.Printf("(Pot: %d)\n", table.GameState.Pot)

		fmt.Println("\nBalances:")
		for _, p := range table.Players {
			fmt.Printf("  %s: %d\n", p.Name, p.Balance)
		}

		fmt.Print("\nPlay again? (y/n): ")
		scanner.Scan()
		if strings.TrimSpace(scanner.Text()) != "y" {
			break
		}
		fmt.Println()
	}

	fmt.Println("Thanks for playing!")
}
