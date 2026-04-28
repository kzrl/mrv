// Package model implements the model.LLM interface for OpenAI-compatible endpoints
// (e.g. llama.cpp server running on localhost:8080).
package model

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	"google.golang.org/genai"

	adkmodel "google.golang.org/adk/model"
)

// OpenAIModel adapts an OpenAI-compatible HTTP API to the ADK model.LLM interface.
type OpenAIModel struct {
	client *openai.Client
	name   string
}

// New creates a new OpenAIModel.
// baseURL should point to the OpenAI-compatible server (e.g. "http://localhost:8080/v1").
// modelName is sent as the model field in requests.
func New(baseURL, modelName string) *OpenAIModel {
	cfg := openai.DefaultConfig("none") // llama.cpp doesn't need an API key
	cfg.BaseURL = baseURL
	return &OpenAIModel{
		client: openai.NewClientWithConfig(cfg),
		name:   modelName,
	}
}

// Name returns the model name.
func (m *OpenAIModel) Name() string {
	return m.name
}

// GenerateContent implements model.LLM. It converts the ADK request to OpenAI
// format, calls the server, and converts the response back.
func (m *OpenAIModel) GenerateContent(ctx context.Context, req *adkmodel.LLMRequest, stream bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		msgs, err := contentsToOpenAI(req)
		if err != nil {
			yield(nil, fmt.Errorf("model: build messages: %w", err))
			return
		}

		tools := extractTools(req)

		oaiReq := openai.ChatCompletionRequest{
			Model:    m.name,
			Messages: msgs,
			Tools:    tools,
		}

		if stream {
			m.runStream(ctx, oaiReq, yield)
		} else {
			m.runSync(ctx, oaiReq, yield)
		}
	}
}

// runSync calls the API without streaming and yields a single LLMResponse.
func (m *OpenAIModel) runSync(ctx context.Context, req openai.ChatCompletionRequest, yield func(*adkmodel.LLMResponse, error) bool) {
	resp, err := m.client.CreateChatCompletion(ctx, req)
	if err != nil {
		yield(nil, fmt.Errorf("model: chat completion: %w", err))
		return
	}
	if len(resp.Choices) == 0 {
		yield(nil, fmt.Errorf("model: empty response"))
		return
	}
	llmResp, err := openaiChoiceToLLMResponse(resp.Choices[0].Message, resp.Choices[0].FinishReason, false)
	if err != nil {
		yield(nil, err)
		return
	}
	llmResp.TurnComplete = true
	yield(llmResp, nil)
}

// runStream calls the API with streaming enabled and yields partial and final responses.
func (m *OpenAIModel) runStream(ctx context.Context, req openai.ChatCompletionRequest, yield func(*adkmodel.LLMResponse, error) bool) {
	req.Stream = true
	stream, err := m.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		yield(nil, fmt.Errorf("model: stream: %w", err))
		return
	}
	defer stream.Close()

	var fullText strings.Builder
	var toolCalls []openai.ToolCall
	var finishReason openai.FinishReason

	for {
		chunk, err := stream.Recv()
		if err != nil {
			// io.EOF signals end of stream; treat as turn complete
			break
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		finishReason = chunk.Choices[0].FinishReason

		if delta.Content != "" {
			fullText.WriteString(delta.Content)
			// Yield partial text response.
			text := delta.Content
			yield(&adkmodel.LLMResponse{
				Content: &genai.Content{
					Role:  genai.RoleModel,
					Parts: []*genai.Part{genai.NewPartFromText(text)},
				},
				Partial: true,
			}, nil)
		}

		// Accumulate streaming tool calls.
		for _, tc := range delta.ToolCalls {
			mergeStreamingToolCall(&toolCalls, tc)
		}
	}

	// Yield final response.
	msg := openai.ChatCompletionMessage{
		Role:      openai.ChatMessageRoleAssistant,
		Content:   fullText.String(),
		ToolCalls: toolCalls,
	}
	final, err := openaiChoiceToLLMResponse(msg, finishReason, true)
	if err != nil {
		yield(nil, err)
		return
	}
	yield(final, nil)
}

// contentsToOpenAI converts ADK LLMRequest contents + config to OpenAI messages.
func contentsToOpenAI(req *adkmodel.LLMRequest) ([]openai.ChatCompletionMessage, error) {
	var msgs []openai.ChatCompletionMessage

	// Prepend system instruction if present.
	if req.Config != nil && req.Config.SystemInstruction != nil {
		text := contentText(req.Config.SystemInstruction)
		if text != "" {
			msgs = append(msgs, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: text,
			})
		}
	}

	// pendingCallIDs is a queue of tool_call IDs generated for the most recent
	// model message's function calls. Tool response messages consume them in order.
	var pendingCallIDs []string

	for i, c := range req.Contents {
		if c == nil {
			continue
		}
		switch c.Role {
		case genai.RoleModel, "":
			var textParts []string
			var toolCalls []openai.ToolCall
			pendingCallIDs = nil // reset before processing this model turn

			for j, p := range c.Parts {
				if p == nil {
					continue
				}
				if p.Text != "" {
					textParts = append(textParts, p.Text)
				} else if p.FunctionCall != nil {
					id := fmt.Sprintf("call_%d_%d", i, j)
					pendingCallIDs = append(pendingCallIDs, id)
					argsJSON, err := json.Marshal(p.FunctionCall.Args)
					if err != nil {
						return nil, fmt.Errorf("marshal function args: %w", err)
					}
					toolCalls = append(toolCalls, openai.ToolCall{
						ID:   id,
						Type: openai.ToolTypeFunction,
						Function: openai.FunctionCall{
							Name:      p.FunctionCall.Name,
							Arguments: string(argsJSON),
						},
					})
				}
			}

			msg := openai.ChatCompletionMessage{
				Role:      openai.ChatMessageRoleAssistant,
				Content:   strings.Join(textParts, "\n"),
				ToolCalls: toolCalls,
			}
			msgs = append(msgs, msg)

		case genai.RoleUser:
			// User messages may contain text or function responses (tool results).
			hasFuncResp := false
			for _, p := range c.Parts {
				if p != nil && p.FunctionResponse != nil {
					hasFuncResp = true
					break
				}
			}

			if hasFuncResp {
				// Convert each FunctionResponse to a separate OpenAI tool message.
				for _, p := range c.Parts {
					if p == nil || p.FunctionResponse == nil {
						continue
					}
					callID := ""
					if len(pendingCallIDs) > 0 {
						callID = pendingCallIDs[0]
						pendingCallIDs = pendingCallIDs[1:]
					}
					respJSON, err := json.Marshal(p.FunctionResponse.Response)
					if err != nil {
						return nil, fmt.Errorf("marshal function response: %w", err)
					}
					msgs = append(msgs, openai.ChatCompletionMessage{
						Role:       openai.ChatMessageRoleTool,
						ToolCallID: callID,
						Content:    string(respJSON),
					})
				}
			} else {
				msgs = append(msgs, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: contentText(c),
				})
			}

		case "tool":
			// Some ADK versions use "tool" role for function responses.
			for _, p := range c.Parts {
				if p == nil || p.FunctionResponse == nil {
					continue
				}
				callID := ""
				if len(pendingCallIDs) > 0 {
					callID = pendingCallIDs[0]
					pendingCallIDs = pendingCallIDs[1:]
				}
				respJSON, err := json.Marshal(p.FunctionResponse.Response)
				if err != nil {
					return nil, fmt.Errorf("marshal function response: %w", err)
				}
				msgs = append(msgs, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					ToolCallID: callID,
					Content:    string(respJSON),
				})
			}
		}
	}

	return msgs, nil
}

// extractTools converts genai tool declarations in the request config to OpenAI tools.
func extractTools(req *adkmodel.LLMRequest) []openai.Tool {
	if req.Config == nil || len(req.Config.Tools) == 0 {
		return nil
	}
	var tools []openai.Tool
	for _, t := range req.Config.Tools {
		if t == nil {
			continue
		}
		for _, fd := range t.FunctionDeclarations {
			if fd == nil {
				continue
			}
			tools = append(tools, openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        fd.Name,
					Description: fd.Description,
					Parameters:  schemaToMap(fd.Parameters),
				},
			})
		}
	}
	return tools
}

// openaiChoiceToLLMResponse converts an OpenAI message to an ADK LLMResponse.
func openaiChoiceToLLMResponse(msg openai.ChatCompletionMessage, finishReason openai.FinishReason, turnComplete bool) (*adkmodel.LLMResponse, error) {
	var parts []*genai.Part

	if msg.Content != "" {
		parts = append(parts, genai.NewPartFromText(msg.Content))
	}

	for _, tc := range msg.ToolCalls {
		var args map[string]any
		if tc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("unmarshal tool call args for %s: %w", tc.Function.Name, err)
			}
		}
		parts = append(parts, genai.NewPartFromFunctionCall(tc.Function.Name, args))
	}

	if len(parts) == 0 {
		parts = append(parts, genai.NewPartFromText(""))
	}

	return &adkmodel.LLMResponse{
		Content: &genai.Content{
			Role:  genai.RoleModel,
			Parts: parts,
		},
		TurnComplete: turnComplete,
		FinishReason: finishReasonToGenai(finishReason),
	}, nil
}

// contentText extracts all text parts from a genai.Content as a single string.
func contentText(c *genai.Content) string {
	if c == nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range c.Parts {
		if p != nil && p.Text != "" {
			sb.WriteString(p.Text)
		}
	}
	return sb.String()
}

// schemaToMap converts a genai.Schema to a JSON-schema-compatible map for OpenAI.
func schemaToMap(s *genai.Schema) map[string]any {
	if s == nil {
		return map[string]any{"type": "object"}
	}
	result := map[string]any{}
	if s.Type != "" {
		result["type"] = strings.ToLower(string(s.Type))
	}
	if s.Description != "" {
		result["description"] = s.Description
	}
	if len(s.Properties) > 0 {
		props := map[string]any{}
		for k, v := range s.Properties {
			props[k] = schemaToMap(v)
		}
		result["properties"] = props
	}
	if len(s.Required) > 0 {
		result["required"] = s.Required
	}
	if s.Items != nil {
		result["items"] = schemaToMap(s.Items)
	}
	if len(s.Enum) > 0 {
		result["enum"] = s.Enum
	}
	return result
}

// mergeStreamingToolCall accumulates a streaming tool call delta into the slice.
func mergeStreamingToolCall(calls *[]openai.ToolCall, delta openai.ToolCall) {
	if delta.Index == nil {
		return
	}
	idx := *delta.Index
	// Streaming delivers tool calls as deltas indexed by position.
	for len(*calls) <= idx {
		*calls = append(*calls, openai.ToolCall{})
	}
	tc := &(*calls)[idx]
	if delta.ID != "" {
		tc.ID = delta.ID
	}
	if delta.Type != "" {
		tc.Type = delta.Type
	}
	tc.Function.Name += delta.Function.Name
	tc.Function.Arguments += delta.Function.Arguments
}

// finishReasonToGenai maps OpenAI finish reasons to genai equivalents.
func finishReasonToGenai(r openai.FinishReason) genai.FinishReason {
	switch r {
	case openai.FinishReasonStop:
		return genai.FinishReasonStop
	case openai.FinishReasonToolCalls:
		return genai.FinishReasonStop // tool calls are not a separate finish reason in genai
	case openai.FinishReasonLength:
		return genai.FinishReasonMaxTokens
	default:
		return genai.FinishReasonUnspecified
	}
}
