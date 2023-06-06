package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/TikTokTechImmersion/assignment_demo_2023/rpc-server/kitex_gen/rpc"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// IMServiceImpl implements the last service interface defined in the IDL.
type IMServiceImpl struct{}

var (
	invalidCursorErr = errors.New("invalid cursor")
	invalidLimitErr  = errors.New("invalid limit")
)

func (s *IMServiceImpl) Send(ctx context.Context, req *rpc.SendRequest) (*rpc.SendResponse, error) {
	resp := rpc.NewSendResponse()

	userMessage := req.GetMessage()
	if err := ValidateMessage(userMessage); err != nil {
		resp.Code = 1
		resp.Msg = err.Error()
		return resp, err
	}

	sender, receiver := GetSenderReceiver(userMessage)
	chatMessage := &ChatMessage{
		ChatID:   GetNormalisedChatIDFromMessage(userMessage),
		Sender:   sender,
		Receiver: receiver,
		Text:     userMessage.GetText(),
		SentAt:   uint64(userMessage.SendTime),
	}

	db := GetDatabase()
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(chatMessage).Error; err != nil {
			return err
		}

		if err := tx.Where("chat_id = ? AND reverse = true", chatMessage.ChatID).Delete(&ChatCursorCache{}).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		resp.Code = -1
		resp.Msg = "something went wrong..."
		log.Printf("Error when creating message: %+v\n", err)
		return resp, err
	}

	resp.Code, resp.Msg = 0, "success"
	return resp, nil
}

func (s *IMServiceImpl) Pull(ctx context.Context, req *rpc.PullRequest) (*rpc.PullResponse, error) {
	resp := rpc.NewPullResponse()

	if err := ValidateChatID(req.GetChat()); err != nil {
		resp.Code = 1
		resp.Msg = err.Error()
		return resp, err
	} else {
		req.SetChat(GetNormalisedChatID(req.GetChat()))
	}

	if req.GetLimit() < 0 {
		resp.Code = 2
		resp.Msg = invalidLimitErr.Error()
		return resp, invalidLimitErr
	} else if req.GetLimit() == 0 {
		req.SetLimit(10)
	}

	if req.GetCursor() < 0 {
		resp.Code = 3
		resp.Msg = invalidCursorErr.Error()
		return resp, invalidCursorErr
	}

	messages, err := getMessages(req)
	if err != nil {
		resp.Code = -1
		resp.Msg = err.Error()
		return resp, err
	}

	limit := int(req.GetLimit())
	hasMore := false
	respMessages := make([]*rpc.Message, Min(limit, len(messages)))
	for i, msg := range messages {
		if i == limit {
			hasMore = true
			nextCursor := req.GetCursor() + int64(limit)
			resp.SetNextCursor(&nextCursor)
			createChatCursorCache(&msg, req.GetReverse(), nextCursor)
			break
		}
		respMessages[i] = msg.ToResponse()
	}

	resp.SetHasMore(&hasMore)
	resp.SetMessages(respMessages)
	resp.Code, resp.Msg = 0, "success"
	return resp, nil
}

func createChatCursorCache(msg *ChatMessage, reverse bool, cursor int64) {
	chatCursorCache := msg.ToChatCursor(reverse, cursor)
	if err := GetDatabase().Create(chatCursorCache).Error; err != nil {
		var perr *pgconn.PgError
		errors.As(err, &perr)
		if perr != nil && perr.Code != "23505" {
			log.Printf("Error when creating chat cursor cache: %+v\n", err)
		}
	}
}

func getChatCacheCursor(req *rpc.PullRequest) (*ChatCursorCache, error) {
	if req.GetCursor() == 0 {
		return nil, nil
	}

	db := GetDatabase()
	cachedCursor := new(ChatCursorCache)
	if err := db.Where(
		"chat_id = ? AND reverse = ? AND cursor = ?",
		req.GetChat(), req.GetReverse(), req.GetCursor(),
	).First(cachedCursor).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return cachedCursor, err
		} else {
			cachedCursor = nil
		}
	}
	return cachedCursor, nil
}

func getMessages(req *rpc.PullRequest) ([]ChatMessage, error) {
	cachedCursor, err := getChatCacheCursor(req)
	if err != nil {
		return nil, err
	}

	db := GetDatabase()
	sortType := "ASC"
	sortCond := ">="
	if req.GetReverse() {
		sortType = "DESC"
		sortCond = "<="
	}

	var messages []ChatMessage
	limit := int(req.GetLimit())
	if cachedCursor != nil {
		queryCondition := fmt.Sprintf("chat_id = ? AND sent_at %s ?", sortCond)
		if err := db.Where(queryCondition, req.GetChat(), cachedCursor.SentAt).Order(fmt.Sprintf("sent_at %s", sortType)).Limit(limit + 1).Find(&messages).Error; err != nil {
			return nil, err
		}
	} else {
		if err := db.Where("chat_id = ?", req.GetChat()).Order(fmt.Sprintf("sent_at %s", sortType)).Offset(int(req.GetCursor())).Limit(limit + 1).Find(&messages).Error; err != nil {
			return nil, err
		}

		if req.GetCursor() > 0 && len(messages) > 0 {
			createChatCursorCache(&messages[0], req.GetReverse(), req.GetCursor())
		}
	}

	return messages, nil
}
