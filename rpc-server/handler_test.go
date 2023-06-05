package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/TikTokTechImmersion/assignment_demo_2023/rpc-server/kitex_gen/rpc"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
)

type pullArgs struct {
	ctx context.Context
	req *rpc.PullRequest
}

type pullTest struct {
	name               string
	args               pullArgs
	wantErr            error
	wantHasMore        bool
	wantNextCursor     int
	wantResponseLength int
}

func b(val bool) *bool {
	return &val
}

// Reference: https://github.com/golang/go/wiki/SliceTricks#reversing
func reverse[V any](s []V) []V {
	a := make([]V, len(s))
	copy(a, s)
	for i := len(a)/2 - 1; i >= 0; i-- {
		opp := len(a) - 1 - i
		a[i], a[opp] = a[opp], a[i]
	}
	return a
}

func checkMessageEqual(msgA *rpc.Message, msgB *rpc.Message) bool {
	return msgA.GetChat() == msgB.GetChat() &&
		msgA.GetText() == msgB.GetText() &&
		msgA.GetSender() == msgB.GetSender() &&
		msgA.GetSendTime() == msgB.GetSendTime()
}

func checkMessageContents(t *testing.T, req *rpc.PullRequest, resp *rpc.PullResponse, msgsTruth []*rpc.Message) {
	lowerLimit := req.GetCursor()
	upperLimit := Min(lowerLimit+int64(req.GetLimit()), 100)
	if req.GetLimit() == 0 {
		upperLimit = 10
	}

	respMessages := resp.GetMessages()
	for i, msg := range msgsTruth[lowerLimit:upperLimit] {
		if !checkMessageEqual(respMessages[i], msg) {
			t.Fatalf("wrong contents in retrieved messages")
		}
	}
}

func checkPullResponse(tt pullTest, messages, messagesReversed []*rpc.Message) func(*testing.T) {
	return func(t *testing.T) {
		s := &IMServiceImpl{}
		got, err := s.Pull(tt.args.ctx, tt.args.req)
		assert.NotNil(t, got, "expected response non-nil")
		assert.Truef(t, errors.Is(err, tt.wantErr), "expected error: %+v, got: %+v", tt.wantErr, err)
		assert.Truef(t, got.GetHasMore() == tt.wantHasMore, "expected hasMore: %t, got: %t", tt.wantHasMore, got.GetHasMore())
		assert.Truef(t, len(got.GetMessages()) == tt.wantResponseLength, "expected messages length: %d, got: %d", tt.wantResponseLength, len(got.GetMessages()))

		if tt.wantErr == nil {
			msgsTruth := messages
			if tt.args.req.GetReverse() {
				msgsTruth = messagesReversed
			}
			checkMessageContents(t, tt.args.req, got, msgsTruth)
			assert.Truef(t, got.GetCode() == 0, "expected code zero, got: %d", got.GetCode())
		} else {
			assert.Truef(t, got.GetCode() != 0, "expected code non-zero, got: %d", got.GetCode())
		}

		if tt.wantNextCursor == -1 {
			if !assert.Nil(t, got.NextCursor, "expected next cursor nil") {
				t.Logf("expected next cursor nil, got: %d", *got.NextCursor)
			}
		} else {
			if assert.NotNilf(t, got.NextCursor, "expected next cursor non-nil") {
				assert.Truef(t, *got.NextCursor == int64(tt.wantNextCursor), "expected next cursor: %d, got: %d", tt.wantNextCursor, *got.NextCursor)
			}
		}
	}
}

func TestMain(m *testing.M) {
	InitTestDatabase(sqlite.Open("file::memory:?cache=shared"))
	exitCode := m.Run()
	CloseDatabase()
	os.Exit(exitCode)
}

func TestCheckMessageEqual(t *testing.T) {
	tests := []struct {
		name     string
		msgA     *rpc.Message
		msgB     *rpc.Message
		expected bool
	}{
		{
			name:     "default message",
			msgA:     &rpc.Message{},
			msgB:     &rpc.Message{},
			expected: true,
		},
		{
			name:     "same message",
			msgA:     &rpc.Message{Chat: "a:b", Text: "hi", Sender: "a", SendTime: 1},
			msgB:     &rpc.Message{Chat: "a:b", Text: "hi", Sender: "a", SendTime: 1},
			expected: true,
		},
		{
			name:     "different chat",
			msgA:     &rpc.Message{Chat: "a:b", Text: "hi", Sender: "a", SendTime: 1},
			msgB:     &rpc.Message{Chat: "a:c", Text: "hi", Sender: "a", SendTime: 1},
			expected: false,
		},
		{
			name:     "different text",
			msgA:     &rpc.Message{Chat: "a:b", Text: "hi", Sender: "a", SendTime: 1},
			msgB:     &rpc.Message{Chat: "a:b", Text: "bye", Sender: "a", SendTime: 1},
			expected: false,
		},
		{
			name:     "different sender",
			msgA:     &rpc.Message{Chat: "a:b", Text: "hi", Sender: "a", SendTime: 1},
			msgB:     &rpc.Message{Chat: "a:b", Text: "hi", Sender: "b", SendTime: 1},
			expected: false,
		},
		{
			name:     "different send time",
			msgA:     &rpc.Message{Chat: "a:b", Text: "hi", Sender: "a", SendTime: 1},
			msgB:     &rpc.Message{Chat: "a:b", Text: "hi", Sender: "a", SendTime: 0},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, tt.expected == checkMessageEqual(tt.msgA, tt.msgB))
		})
	}
}

func TestIMServiceImpl_Send(t *testing.T) {
	type args struct {
		ctx context.Context
		req *rpc.SendRequest
	}
	tests := []struct {
		name     string
		args     args
		wantCode int
		wantErr  error
	}{
		{
			name: "valid message",
			args: args{
				ctx: context.Background(),
				req: &rpc.SendRequest{
					Message: &rpc.Message{
						Chat:     "a:b",
						Text:     "hi",
						Sender:   "a",
						SendTime: GetTimeNow().UnixMicro(),
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "ensure message being validated - nil message",
			args: args{
				ctx: context.Background(),
				req: &rpc.SendRequest{},
			},
			wantCode: 1,
			wantErr:  invalidMessage,
		},
		{
			name: "ensure message being validated - fields empty",
			args: args{
				ctx: context.Background(),
				req: &rpc.SendRequest{
					Message: &rpc.Message{},
				},
			},
			wantCode: 1,
			wantErr:  invalidChatID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &IMServiceImpl{}
			got, err := s.Send(tt.args.ctx, tt.args.req)
			assert.NotNil(t, got, "expected response non-nil")
			assert.Truef(t, errors.Is(err, tt.wantErr), "expected error: %+v, got: %+v", tt.wantErr, err)
			assert.Truef(t, got.GetCode() == int32(tt.wantCode), "expected code %d, got: %d", tt.wantCode, got.GetCode())
		})
	}
}

func TestIMServiceImpl_Pull(t *testing.T) {
	// Create messages for testing
	db := GetDatabase()
	chatMembers := []string{"pull_a", "pull_b"}
	chatId := strings.Join(chatMembers, ":")
	chatIdReversed := strings.Join([]string{chatMembers[1], chatMembers[0]}, ":")
	messages := make([]*rpc.Message, 0, 100)
	for i := 0; i < 100; i++ {
		sender := chatMembers[rand.Intn(len(chatMembers))]
		receiver := chatMembers[0]
		if receiver == sender {
			receiver = chatMembers[1]
		}
		msg := ChatMessage{
			ChatID:   chatId,
			Text:     fmt.Sprintf("%d", i),
			Sender:   sender,
			Receiver: receiver,
			SentAt:   uint64(GetTimeNow().UnixMicro()),
		}
		if err := db.Create(&msg).Error; err != nil {
			t.Fatalf("Error when creating test messages for pull implementation test: %+v\n", err)
		}
		messages = append(messages, msg.ToResponse())
	}
	messagesReversed := reverse(messages)
	tests := []pullTest{
		{
			name: "pull [0,4]",
			args: pullArgs{
				ctx: context.Background(),
				req: &rpc.PullRequest{
					Chat:  chatId,
					Limit: 5,
				},
			},
			wantErr:            nil,
			wantHasMore:        true,
			wantNextCursor:     5,
			wantResponseLength: 5,
		},
		{
			name: "pull [0,99]",
			args: pullArgs{
				ctx: context.Background(),
				req: &rpc.PullRequest{
					Chat:  chatId,
					Limit: 100,
				},
			},
			wantErr:            nil,
			wantHasMore:        false,
			wantNextCursor:     -1,
			wantResponseLength: 100,
		},
		{
			name: "pull [0,99] chat id reversed",
			args: pullArgs{
				ctx: context.Background(),
				req: &rpc.PullRequest{
					Chat:  chatIdReversed,
					Limit: 100,
				},
			},
			wantErr:            nil,
			wantHasMore:        false,
			wantNextCursor:     -1,
			wantResponseLength: 100,
		},
		{
			name: "pull [0,99]",
			args: pullArgs{
				ctx: context.Background(),
				req: &rpc.PullRequest{
					Chat:    chatId,
					Reverse: b(true),
					Limit:   100,
				},
			},
			wantErr:            nil,
			wantHasMore:        false,
			wantNextCursor:     -1,
			wantResponseLength: 100,
		},
		{
			name: "pull [99,99]",
			args: pullArgs{
				ctx: context.Background(),
				req: &rpc.PullRequest{
					Chat:   chatId,
					Cursor: 99,
					Limit:  1,
				},
			},
			wantErr:            nil,
			wantHasMore:        false,
			wantNextCursor:     -1,
			wantResponseLength: 1,
		},
		{
			name: "pull [99,99] repeated to ensure cache cursor working",
			args: pullArgs{
				ctx: context.Background(),
				req: &rpc.PullRequest{
					Chat:   chatId,
					Cursor: 99,
					Limit:  1,
				},
			},
			wantErr:            nil,
			wantHasMore:        false,
			wantNextCursor:     -1,
			wantResponseLength: 1,
		},
		{
			name: "pull [100,100]",
			args: pullArgs{
				ctx: context.Background(),
				req: &rpc.PullRequest{
					Chat:   chatId,
					Cursor: 100,
					Limit:  1,
				},
			},
			wantErr:            nil,
			wantHasMore:        false,
			wantNextCursor:     -1,
			wantResponseLength: 0,
		},
		{ // should return first error
			name: "pull default values",
			args: pullArgs{
				ctx: context.Background(),
				req: &rpc.PullRequest{},
			},
			wantErr:            invalidChatID,
			wantHasMore:        false,
			wantNextCursor:     -1,
			wantResponseLength: 0,
		},
		{
			name: "pull invalid cursor",
			args: pullArgs{
				ctx: context.Background(),
				req: &rpc.PullRequest{
					Chat:   chatId,
					Cursor: -1,
				},
			},
			wantErr:            invalidCursorErr,
			wantHasMore:        false,
			wantNextCursor:     -1,
			wantResponseLength: 0,
		},
		{
			name: "pull limit = 0 (should default to 10)",
			args: pullArgs{
				ctx: context.Background(),
				req: &rpc.PullRequest{
					Chat: chatId,
				},
			},
			wantErr:            nil,
			wantHasMore:        true,
			wantNextCursor:     10,
			wantResponseLength: 10,
		},
		{
			name: "pull invalid limit = -1",
			args: pullArgs{
				ctx: context.Background(),
				req: &rpc.PullRequest{
					Chat:  chatId,
					Limit: -1,
				},
			},
			wantErr:            invalidLimitErr,
			wantHasMore:        false,
			wantNextCursor:     -1,
			wantResponseLength: 0,
		},
		{
			name: "pull invalid chat id",
			args: pullArgs{
				ctx: context.Background(),
				req: &rpc.PullRequest{
					Chat:  "pull_a:",
					Limit: 1,
				},
			},
			wantErr:            invalidChatID,
			wantHasMore:        false,
			wantNextCursor:     -1,
			wantResponseLength: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, checkPullResponse(tt, messages, messagesReversed))
	}
}

func TestIMServiceImpl_Send_Pull_Normalisation(t *testing.T) {
	chatMembers := []string{"normalisation_a", "normalisation_b"}
	chatId := strings.Join(chatMembers, ":")
	chatIdReversed := strings.Join([]string{chatMembers[1], chatMembers[0]}, ":")
	for i := 0; i < 100; i++ {
		s := &IMServiceImpl{}
		chatIdUsed := chatId
		sender := chatMembers[rand.Intn(len(chatMembers))]
		if i >= 50 {
			chatIdUsed = chatIdReversed
		}
		if _, err := s.Send(context.Background(), &rpc.SendRequest{
			Message: &rpc.Message{
				Chat:     chatIdUsed,
				Text:     fmt.Sprintf("%d", i),
				Sender:   sender,
				SendTime: GetTimeNow().UnixMicro(),
			},
		}); err != nil {
			t.Fatalf("Error when creating test messages for send pull implementation test: %+v\n", err)
		}
	}

	var dbMessages []ChatMessage
	err := GetDatabase().Where("chat_id = ?", chatId).Order("sent_at ASC").Find(&dbMessages).Error
	if err != nil || len(dbMessages) != 100 {
		t.Fatalf("Error when retrieve created test messages for send pull implementation test: %+v, length retrieved: %d\n", err, len(dbMessages))
	}

	messages := make([]*rpc.Message, 0, 100)
	for _, msg := range dbMessages {
		messages = append(messages, msg.ToResponse())
	}
	messagesReversed := reverse(messages)

	tests := []pullTest{
		{
			name: "pull [0,99]",
			args: pullArgs{
				ctx: context.Background(),
				req: &rpc.PullRequest{
					Chat:  chatId,
					Limit: 100,
				},
			},
			wantErr:            nil,
			wantHasMore:        false,
			wantNextCursor:     -1,
			wantResponseLength: 100,
		},
		{
			name: "pull [0,99]",
			args: pullArgs{
				ctx: context.Background(),
				req: &rpc.PullRequest{
					Chat:  chatIdReversed,
					Limit: 100,
				},
			},
			wantErr:            nil,
			wantHasMore:        false,
			wantNextCursor:     -1,
			wantResponseLength: 100,
		}, {
			name: "pull [0,99]",
			args: pullArgs{
				ctx: context.Background(),
				req: &rpc.PullRequest{
					Chat:    chatId,
					Reverse: b(true),
					Limit:   100,
				},
			},
			wantErr:            nil,
			wantHasMore:        false,
			wantNextCursor:     -1,
			wantResponseLength: 100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, checkPullResponse(tt, messages, messagesReversed))
	}
}
