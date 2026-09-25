package course

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-kafka/v3/pkg/kafka"
	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/priyansx01/corn-hub-clone/internal/config"
	"github.com/priyansx01/corn-hub-clone/internal/domain"
	"github.com/priyansx01/corn-hub-clone/internal/event"
)

type Consumer struct {
	cfg config.Config
	svc *Service
	sub message.Subscriber
}

func NewConsumer(cfg config.Config, svc *Service) (*Consumer, error) {
	sub, err := kafka.NewSubscriber(
		kafka.SubscriberConfig{
			Brokers:       []string{cfg.Kafka.Brokers},
			Unmarshaler:   kafka.DefaultMarshaler{},
			ConsumerGroup: "lms-api",
		},
		watermill.NewStdLogger(false, false),
	)
	if err != nil {
		return nil, fmt.Errorf("create subscriber: %w", err)
	}

	return &Consumer{
		cfg: cfg,
		svc: svc,
		sub: sub,
	}, nil
}

func (c *Consumer) Start(ctx context.Context) error {
	messages, err := c.sub.Subscribe(ctx, event.TopicVideoProcessed)
	if err != nil {
		return fmt.Errorf("subscribe to %s: %w", event.TopicVideoProcessed, err)
	}

	log.Printf("🎧 API Consumer started. Listening to %s...", event.TopicVideoProcessed)

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping API consumer...")
			return c.sub.Close()
		case msg, ok := <-messages:
			if !ok {
				return nil
			}
			c.handleMessage(ctx, msg)
		}
	}
}

func (c *Consumer) handleMessage(ctx context.Context, msg *message.Message) {
	var ev event.VideoProcessedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		log.Printf("❌ API Consumer failed to unmarshal message: %v", err)
		msg.Ack()
		return
	}

	log.Printf("⚙️  API Consumer received processed video for module %s. Updating DB...", ev.ModuleID)

	_, err := c.svc.db.ExecContext(ctx, `
		UPDATE modules 
		SET status = $1, hls_url = $2, duration_seconds = $3, updated_at = NOW() 
		WHERE id = $4 AND course_id = $5
	`, domain.CourseStatusReady, ev.HLSUrl, ev.DurationSeconds, ev.ModuleID, ev.CourseID)

	if err != nil {
		log.Printf("❌ API Consumer failed to update module %s: %v", ev.ModuleID, err)
		msg.Nack()
		return
	}

	// Update course status to ready if this is the first module, or simply update course thumbnail if not set.
	// For now, let's just make the course ready if it was pending or processing.
	_, err = c.svc.db.ExecContext(ctx, `
		UPDATE courses 
		SET status = $1, hls_url = COALESCE(hls_url, $2), thumbnail_url = COALESCE(thumbnail_url, $3), updated_at = NOW() 
		WHERE id = $4 AND status IN ($5, $6)
	`, domain.CourseStatusReady, ev.HLSUrl, ev.ThumbnailUrl, ev.CourseID, domain.CourseStatusPending, domain.CourseStatusProcessing)

	if err != nil {
		log.Printf("⚠ API Consumer failed to update course %s: %v", ev.CourseID, err)
	}

	log.Printf("✅ API Consumer successfully updated module %s.", ev.ModuleID)
	msg.Ack()
}
