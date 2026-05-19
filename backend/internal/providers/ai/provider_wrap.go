package ai

import (
	"context"

	"github.com/trademind-ai/trademind/backend/internal/providers/ai/compatclient"
	"github.com/trademind-ai/trademind/backend/internal/providers/ai/errmap"
)

type compatCaller interface {
	Name() string
	Chat(ctx context.Context, req compatclient.Request) (*compatclient.Result, error)
}

type compatProvider struct {
	inner compatCaller
}

func (p *compatProvider) Name() string {
	if p == nil || p.inner == nil {
		return ""
	}
	return p.inner.Name()
}

func (p *compatProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if p == nil || p.inner == nil {
		return nil, errmapNil("AI Provider")
	}
	creq := compatclient.Request{
		Model:          req.Model,
		Messages:       toCompatMessages(req.Messages),
		Temperature:    req.Temperature,
		MaxTokens:      req.MaxTokens,
		ResponseFormat: responseFormatType(req),
	}
	res, err := p.inner.Chat(ctx, creq)
	if err != nil {
		return nil, err
	}
	model := res.Model
	if model == "" {
		model = req.Model
	}
	return &ChatResponse{
		Content:      res.Content,
		Model:        model,
		Raw:          res.Raw,
		InputTokens:  res.InputTokens,
		OutputTokens: res.OutputTokens,
	}, nil
}

func errmapNil(label string) error {
	return errmap.MapChatError(label, compatclient.ErrNilClient())
}
