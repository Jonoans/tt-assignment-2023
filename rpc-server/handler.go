package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/TikTokTechImmersion/assignment_demo_2023/rpc-server/kitex_gen/rpc"
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

		if err := tx.Where("chat_id = ?", chatMessage.ChatID).Delete(&ChatCursorCache{}).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		resp.Code = -1
		resp.Msg = "something went wrong..."
		log.Printf("Error when creating message: %+v\n", err)
		return resp, err
	}

	resp.Msg = "success"
	return resp, nil
}

func (s *IMServiceImpl) Pull(ctx context.Context, req *rpc.PullRequest) (*rpc.PullResponse, error) {
	resp := rpc.NewPullResponse()

	db := GetDatabase()
	var cachedCursor *ChatCursorCache
	if req.GetCursor() < 0 {
		resp.Code = 1
		resp.Msg = invalidCursorErr.Error()
		return resp, invalidCursorErr
	} else if req.GetCursor() > 0 {
		cachedCursor = new(ChatCursorCache)
		if err := db.First(
			&cachedCursor,
			&ChatCursorCache{
				ChatID:  req.GetChat(),
				Reverse: req.GetReverse(),
				Cursor:  req.GetCursor(),
			},
		).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				resp.Code = -1
				resp.Msg = "something went wrong..."
				return resp, err
			} else {
				cachedCursor = nil
			}
		}
	}

	if req.GetLimit() <= 0 {
		resp.Code = 2
		resp.Msg = invalidLimitErr.Error()
		return resp, invalidLimitErr
	}

	if err := ValidateChatID(req.GetChat()); err != nil {
		resp.Code = 3
		resp.Msg = err.Error()
		return resp, err
	} else {
		req.SetChat(GetNormalisedChatID(req.GetChat()))
	}

	var sortType = "ASC"
	if req.GetReverse() {
		sortType = "DESC"
	}
	limit := int(req.GetLimit())

	var messages []ChatMessage
	if cachedCursor != nil {
		if err := db.Where("chat_id = ? AND sent_at >= ?", req.GetChat(), cachedCursor.SentAt).Order(fmt.Sprintf("sent_at %s", sortType)).Limit(limit + 1).Find(&messages).Error; err != nil {
			resp.Code = -1
			resp.Msg = "something went wrong..."
			log.Printf("Error when pulling message: %+v\n", err)
			return resp, err
		}
	} else {
		if err := db.Where("chat_id = ?", req.GetChat()).Order(fmt.Sprintf("sent_at %s", sortType)).Offset(int(req.GetCursor())).Limit(limit + 1).Find(&messages).Error; err != nil {
			resp.Code = -1
			resp.Msg = "something went wrong..."
			log.Printf("Error when pulling message: %+v\n", err)
			return resp, err
		}

		if req.GetCursor() > 0 && len(messages) > 0 {
			createChatCursorCache(&messages[0], req.GetReverse(), req.GetCursor())
		}
	}

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
