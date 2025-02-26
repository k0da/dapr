/*
Copyright 2021 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	clocklib "github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
)

var startOfTime = time.Date(2022, 1, 1, 12, 0, 0, 0, time.UTC)

func TestHealthCheck(t *testing.T) {
	clock := clocklib.NewMock()
	clock.Set(startOfTime)

	t.Run("unhealthy endpoint custom interval 1, failure threshold 2s", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch := StartEndpointHealthCheck(ctx, "none",
			WithClock(clock),
			WithInterval(time.Second),
			WithFailureThreshold(2),
			WithInitialDelay(0),
		)
		runtime.Gosched()

		// Sleep on the wall clock for a few ms to allow the background goroutine to get in sync
		time.Sleep(50 * time.Millisecond)

		// Nothing happens for the first second
		clock.Add(time.Second)
		assertNoHealthSignal(t, ch)

		// Sleep on the wall clock for a few ms to allow the background goroutine to get in sync
		time.Sleep(50 * time.Millisecond)

		// Get a signal after the next tick
		clock.Add(time.Second)
		healthy := assertHealthSignal(t, ch)
		assert.False(t, healthy)
	})

	t.Run("unhealthy endpoint custom interval 1s, failure threshold 1, initial delay 2s", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch := StartEndpointHealthCheck(ctx, "none",
			WithClock(clock),
			WithInterval(time.Second),
			WithFailureThreshold(1),
			WithInitialDelay(time.Second*2),
		)
		runtime.Gosched()

		// Nothing happens for the first 2s
		for i := 0; i < 2; i++ {
			// Sleep on the wall clock for a few ms to allow the background goroutine to get in sync
			time.Sleep(50 * time.Millisecond)
			clock.Add(time.Second)
			assertNoHealthSignal(t, ch)
		}

		// Sleep on the wall clock for a few ms to allow the background goroutine to get in sync
		time.Sleep(50 * time.Millisecond)

		// Get a signal after the next tick
		clock.Add(time.Second)
		healthy := assertHealthSignal(t, ch)
		assert.False(t, healthy)
	})

	t.Run("unhealthy endpoint custom interval 1s, failure threshold 2, initial delay 2s", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch := StartEndpointHealthCheck(ctx, "none",
			WithClock(clock),
			WithInterval(time.Second*1),
			WithFailureThreshold(2),
			WithInitialDelay(time.Second*2),
		)
		runtime.Gosched()

		// Sleep on the wall clock for a few ms to allow the background goroutine to get in sync
		time.Sleep(50 * time.Millisecond)

		// Nothing happens for the first 3s
		for i := 0; i < 3; i++ {
			clock.Add(time.Second)
			assertNoHealthSignal(t, ch)
		}

		// Sleep on the wall clock for a few ms to allow the background goroutine to get in sync
		time.Sleep(50 * time.Millisecond)

		// Get a signal after the next tick
		clock.Add(time.Second)
		healthy := assertHealthSignal(t, ch)
		assert.False(t, healthy)
	})
}

func TestApplyOptions(t *testing.T) {
	t.Run("valid defaults", func(t *testing.T) {
		opts := healthCheckOptions{}
		applyDefaults(&opts)

		assert.Equal(t, opts.failureThreshold, int32(failureThreshold))
		assert.Equal(t, opts.initialDelay, initialDelay)
		assert.Equal(t, opts.interval, interval)
		assert.Equal(t, opts.requestTimeout, requestTimeout)
		assert.Equal(t, opts.successStatusCode, successStatusCode)
	})

	t.Run("valid custom options", func(t *testing.T) {
		opts := healthCheckOptions{}
		applyDefaults(&opts)

		customOpts := []Option{
			WithFailureThreshold(10),
			WithInitialDelay(time.Second * 11),
			WithInterval(time.Second * 12),
			WithRequestTimeout(time.Second * 13),
			WithSuccessStatusCode(201),
		}
		for _, o := range customOpts {
			o(&opts)
		}
		assert.Equal(t, opts.failureThreshold, int32(10))
		assert.Equal(t, opts.initialDelay, time.Second*11)
		assert.Equal(t, opts.interval, time.Second*12)
		assert.Equal(t, opts.requestTimeout, time.Second*13)
		assert.Equal(t, opts.successStatusCode, 201)
	})
}

type testServer struct {
	statusCode    int
	numberOfCalls atomic.Int64
}

func (t *testServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.numberOfCalls.Add(1)
	w.WriteHeader(t.statusCode)
	w.Write([]byte(""))
}

func TestResponses(t *testing.T) {
	clock := clocklib.NewMock()
	clock.Set(startOfTime)

	t.Run("default success status", func(t *testing.T) {
		server := httptest.NewServer(&testServer{
			statusCode: 200,
		})
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch := StartEndpointHealthCheck(ctx, server.URL,
			WithClock(clock),
			WithInitialDelay(0),
			WithFailureThreshold(1),
		)
		runtime.Gosched()

		// Sleep on the wall clock for a few ms to allow the background goroutine to get in sync
		time.Sleep(50 * time.Millisecond)
		clock.Add(5 * time.Second)
		healthy := assertHealthSignal(t, ch)
		assert.True(t, healthy)
	})

	t.Run("custom success status", func(t *testing.T) {
		server := httptest.NewServer(&testServer{
			statusCode: 201,
		})
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch := StartEndpointHealthCheck(ctx, server.URL,
			WithClock(clock),
			WithInitialDelay(0),
			WithFailureThreshold(1),
			WithSuccessStatusCode(201),
		)
		runtime.Gosched()

		// Sleep on the wall clock for a few ms to allow the background goroutine to get in sync
		time.Sleep(50 * time.Millisecond)
		clock.Add(5 * time.Second)
		healthy := assertHealthSignal(t, ch)
		assert.True(t, healthy)
	})

	t.Run("test fail", func(t *testing.T) {
		server := httptest.NewServer(&testServer{
			statusCode: 500,
		})
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch := StartEndpointHealthCheck(ctx, server.URL,
			WithClock(clock),
			WithInitialDelay(0),
			WithFailureThreshold(1),
		)
		runtime.Gosched()

		// Sleep on the wall clock for a few ms to allow the background goroutine to get in sync
		time.Sleep(50 * time.Millisecond)
		clock.Add(5 * time.Second)
		healthy := assertHealthSignal(t, ch)
		assert.False(t, healthy)
	})

	t.Run("test app recovery", func(t *testing.T) {
		test := &testServer{
			statusCode: 500,
		}
		server := httptest.NewServer(test)
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch := StartEndpointHealthCheck(ctx, server.URL,
			WithClock(clock),
			WithInitialDelay(0),
			WithFailureThreshold(1),
		)
		runtime.Gosched()

		for i := 0; i <= 1; i++ {
			// Sleep on the wall clock for a few ms to allow the background goroutine to get in sync
			time.Sleep(50 * time.Millisecond)
			clock.Add(5 * time.Second)
			healthy := assertHealthSignal(t, ch)
			if i == 0 {
				assert.False(t, healthy)
				test.statusCode = 200
			} else {
				assert.True(t, healthy)
			}
		}
	})
}

func assertHealthSignal(t *testing.T, ch <-chan bool) bool {
	t.Helper()
	runtime.Gosched()
	select {
	case v := <-ch:
		return v
	case <-time.After(200 * time.Millisecond):
		t.Fatal("did not receive a signal in 200ms")
	}
	return false
}

func assertNoHealthSignal(t *testing.T, ch <-chan bool) {
	t.Helper()

	// The signal is sent in a background goroutine, so we need to use a wall clock here
	runtime.Gosched()
	select {
	case <-ch:
		t.Fatal("received unexpected signal")
	case <-time.After(200 * time.Millisecond):
		// all good
	}
}
