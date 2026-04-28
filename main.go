package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	adkagent "google.golang.org/adk/agent"
	"google.golang.org/genai"

	mrvagent "mrv/internal/agent"
	"mrv/internal/config"
	mrvmodel "mrv/internal/model"
	mrvui "mrv/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mrv: config: %v\n", err)
		os.Exit(1)
	}

	llm := mrvmodel.New(cfg.Model.URL, cfg.Model.Name)

	setup, err := mrvagent.New(llm, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mrv: init agent: %v\n", err)
		os.Exit(1)
	}

	inputCh := make(chan string, 1)
	agentCh := make(chan mrvui.Message, 64)

	go runAgentLoop(setup, inputCh, agentCh)

	p := tea.NewProgram(
		mrvui.New(inputCh, agentCh),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "mrv: ui: %v\n", err)
		os.Exit(1)
	}
}

func runAgentLoop(setup *mrvagent.Setup, inputCh <-chan string, agentCh chan<- mrvui.Message) {
	runCfg := adkagent.RunConfig{}

	for userText := range inputCh {
		ctx := context.Background()
		msg := genai.NewContentFromText(userText, genai.RoleUser)

		for event, err := range setup.Runner.Run(
			ctx,
			"local",
			setup.SessionID,
			msg,
			runCfg,
		) {
			if err != nil {
				agentCh <- mrvui.Message{Author: "error", Text: err.Error()}
				break
			}
			if event == nil {
				continue
			}

			if event.Content != nil {
				author := event.Author
				if author == "" {
					author = "mrv"
				}
				for _, part := range event.Content.Parts {
					if part == nil {
						continue
					}
					if part.Text != "" {
						agentCh <- mrvui.Message{Author: author, Text: part.Text}
					} else if part.FunctionCall != nil {
						agentCh <- mrvui.Message{
							Author: "tool:" + part.FunctionCall.Name,
							Text:   formatArgs(part.FunctionCall.Args),
						}
					}
				}
			}
		}

		agentCh <- mrvui.Message{Done: true}
	}
}

func formatArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	var parts []string
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, " ")
}
