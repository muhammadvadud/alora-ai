// Package cli agent uchun interaktiv terminal (REPL) beradi.
package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/muhammadvadud/alora-agent/internal/agent"
)

const sessionID = "cli"

// Run agent bilan suhbat siklini boshlaydi. ctx bekor qilinganda toza to'xtaydi.
func Run(ctx context.Context, chat *agent.Chat) error {
	fmt.Println("AI agent tayyor. Chiqish uchun 'exit' yozing.")
	reader := bufio.NewReader(os.Stdin)

	for {
		if ctx.Err() != nil {
			return nil
		}
		fmt.Print("\nSiz: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			return nil
		}

		render(chat.Stream(ctx, sessionID, input))
	}
}

// render Event oqimini terminalga chizadi.
func render(events <-chan agent.Event) {
	renderTo(events, os.Stdout, os.Stderr)
}

func renderTo(events <-chan agent.Event, stdout, stderr io.Writer) {
	const dim, reset = "\033[2m", "\033[0m"
	var (
		printedLabel        bool
		printingThinking    bool
		thinkingEndsNewline bool
	)

	endThinking := func() {
		if !printingThinking {
			return
		}
		if !thinkingEndsNewline {
			fmt.Fprintln(stdout)
		}
		printingThinking = false
	}

	for ev := range events {
		switch ev.Kind {
		case agent.KindThinking:
			// Reasoning token/chunk bo'lib keladi; belgini har chunk'ga qo'ysak so'zlar
			// "F💭 oyd..." ko'rinishida parchalanadi. Uni oqim boshida bir marta chizamiz.
			if !printingThinking {
				fmt.Fprintf(stdout, "%s💭 %s", dim, reset)
				printingThinking = true
			}
			fmt.Fprintf(stdout, "%s%s%s", dim, ev.Content, reset)
			thinkingEndsNewline = strings.HasSuffix(ev.Content, "\n")
		case agent.KindToolCall:
			endThinking()
			fmt.Fprintf(stdout, "\n%s  ⚙ %s%s\n", dim, ev.Tool, reset) // nom aniqlandi
		case agent.KindToolArgs:
			endThinking()
			fmt.Fprintf(stdout, "%s     %s → %s%s\n", dim, ev.Tool, ev.Args, reset) // argumentlar
		case agent.KindToolResult:
			endThinking()
			fmt.Fprintf(stdout, "%s  ↳ %s: %s%s\n", dim, ev.Tool, truncate(ev.Result, 200), reset)
		case agent.KindContent:
			endThinking()
			if !printedLabel {
				fmt.Fprint(stdout, "Agent: ")
				printedLabel = true
			}
			fmt.Fprint(stdout, ev.Content)
		case agent.KindError:
			endThinking()
			fmt.Fprintf(stderr, "\nxato: %s\n", ev.Err)
		case agent.KindDone:
			endThinking()
			fmt.Fprintln(stdout)
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
