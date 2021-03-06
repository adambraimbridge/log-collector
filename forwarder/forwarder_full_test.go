package forwarder

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type s3ServiceMock struct {
	sync.RWMutex
	cache []string
}

var s3Mock = &s3ServiceMock{}

func (s3 *s3ServiceMock) Put(obj string) error {
	obj = strings.Replace(obj, "dispatch", "safe", -1)
	obj = strings.Replace(obj, "error", "dispatch", -1)
	s3.Lock()
	s3.cache = append(s3.cache, obj)
	s3.Unlock()
	return nil
}

func init() {
	Env = "dummy"
	Workers = 8
	ChanBuffer = 256
	Batchsize = 10
	Batchtimer = 5
	Bucket = "testbucket"

	NewS3Service = func(string, string, string) (S3Service, error) {
		return s3Mock, nil
	}
}

func Test_Forwarder(t *testing.T) {
	in, out := io.Pipe()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		Forward(in)
		wg.Done()
	}()

	messageCount := 100
	for i := 0; i < messageCount; i++ {
		out.Write([]byte(`127.0.0.1 - - [21/Apr/2015:12:15:34 +0000] "GET /eom-file/all/e09b49d6-e1fa-11e4-bb7f-00144feab7de HTTP/1.1" 200 53706 919 919` + "\n"))
	}

	if err := out.Close(); err != nil {
		assert.Fail(t, "Error closing the pipe writer %v", err)
	}

	if waitTimeout(&wg, 2*time.Second) {
		assert.Fail(t, "Forwarder should have been stopped on pipe close")
	}

	s3Mock.RLock()
	s3Len := len(s3Mock.cache)
	s3Mock.RUnlock()

	assert.Equal(t, messageCount/Batchsize, s3Len)
}

func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}
