package rabbitmq

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublisherCloseAndConcurrentPublishWhenClosed(t *testing.T) {
	p := &Publisher{url: "amqp://127.0.0.1:1/"}
	p.Close()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			err := p.Publish(context.Background(), "test", []byte(`{}`), "expense_created")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "closed")
		}()
		go func() {
			defer wg.Done()
			p.Close()
		}()
	}
	wg.Wait()
}
