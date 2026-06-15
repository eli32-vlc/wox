package common

import "testing"

// TestIsAllToolCallsCompleted locks the contract that drives the chat loop's
// recursion gate. The chat must recurse whenever a turn had tool calls and
// every call has reached a terminal state (succeeded or failed). A failed
// tool call still counts as "completed" — the model needs to see the error
// in the next turn, so the conversation must continue.
func TestIsAllToolCallsCompleted(t *testing.T) {
	tests := []struct {
		name string
		data ChatStreamData
		want bool
	}{
		{
			name: "empty tool calls does not count as a tool turn",
			data: ChatStreamData{Status: ChatStreamStatusFinished, ToolCalls: nil},
			want: false,
		},
		{
			name: "single succeeded tool call is completed",
			data: ChatStreamData{
				Status:    ChatStreamStatusFinished,
				ToolCalls: []ToolCallInfo{{Status: ToolCallStatusSucceeded}},
			},
			want: true,
		},
		{
			name: "single failed tool call is completed",
			data: ChatStreamData{
				Status:    ChatStreamStatusFinished,
				ToolCalls: []ToolCallInfo{{Status: ToolCallStatusFailed}},
			},
			want: true,
		},
		{
			name: "mix of succeeded and failed is completed",
			data: ChatStreamData{
				Status: ChatStreamStatusFinished,
				ToolCalls: []ToolCallInfo{
					{Status: ToolCallStatusSucceeded},
					{Status: ToolCallStatusFailed},
					{Status: ToolCallStatusSucceeded},
				},
			},
			want: true,
		},
		{
			name: "a still-pending tool call is not completed",
			data: ChatStreamData{
				Status: ChatStreamStatusFinished,
				ToolCalls: []ToolCallInfo{
					{Status: ToolCallStatusSucceeded},
					{Status: ToolCallStatusPending},
				},
			},
			want: false,
		},
		{
			name: "a still-running tool call is not completed",
			data: ChatStreamData{
				Status: ChatStreamStatusFinished,
				ToolCalls: []ToolCallInfo{
					{Status: ToolCallStatusRunning},
				},
			},
			want: false,
		},
		{
			name: "a still-streaming tool call is not completed",
			data: ChatStreamData{
				Status: ChatStreamStatusFinished,
				ToolCalls: []ToolCallInfo{
					{Status: ToolCallStatusStreaming},
				},
			},
			want: false,
		},
		{
			name: "non-finished status is not completed",
			data: ChatStreamData{
				Status:    ChatStreamStatusStreamed,
				ToolCalls: []ToolCallInfo{{Status: ToolCallStatusSucceeded}},
			},
			want: false,
		},
		{
			name: "error status is not completed (chat terminates, no recursion)",
			data: ChatStreamData{
				Status:    ChatStreamStatusError,
				ToolCalls: []ToolCallInfo{{Status: ToolCallStatusFailed}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.data.IsAllToolCallsCompleted()
			if got != tt.want {
				t.Errorf("IsAllToolCallsCompleted() = %v, want %v", got, tt.want)
			}
		})
	}
}
