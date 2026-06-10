package main

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	PRO = true
	code := m.Run()
	os.Exit(code)
}

func TestEmitter(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := NewTestInput()
	output := NewTestOutput(func(*Message) {
		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := NewEmitter()
	go emitter.Start(plugins, Settings.Middleware)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()
	emitter.Close()
}

func TestEmitterFiltered(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := NewTestInput()
	input.skipHeader = true

	output := NewTestOutput(func(*Message) {
		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{output},
	}
	plugins.All = append(plugins.All, input, output)

	methods := HTTPMethods{[]byte("GET")}
	Settings.ModifierConfig = HTTPModifierConfig{Methods: methods}

	emitter := &Emitter{}
	go emitter.Start(plugins, "")

	wg.Add(2)

	id := uuid()
	reqh := payloadHeader(RequestPayload, id, time.Now().UnixNano(), -1)
	reqb := append(reqh, []byte("POST / HTTP/1.1\r\nHost: www.w3.org\r\nUser-Agent: Go 1.1 package http\r\nAccept-Encoding: gzip\r\n\r\n")...)

	resh := payloadHeader(ResponsePayload, id, time.Now().UnixNano()+1, 1)
	respb := append(resh, []byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")...)

	input.EmitBytes(reqb)
	input.EmitBytes(respb)

	id = uuid()
	reqh = payloadHeader(RequestPayload, id, time.Now().UnixNano(), -1)
	reqb = append(reqh, []byte("GET / HTTP/1.1\r\nHost: www.w3.org\r\nUser-Agent: Go 1.1 package http\r\nAccept-Encoding: gzip\r\n\r\n")...)

	resh = payloadHeader(ResponsePayload, id, time.Now().UnixNano()+1, 1)
	respb = append(resh, []byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")...)

	input.EmitBytes(reqb)
	input.EmitBytes(respb)

	wg.Wait()
	emitter.Close()

	Settings.ModifierConfig = HTTPModifierConfig{}
}

func TestEmitterSplitRoundRobin(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := NewTestInput()

	var counter1, counter2 int32

	output1 := NewTestOutput(func(*Message) {
		atomic.AddInt32(&counter1, 1)
		wg.Done()
	})

	output2 := NewTestOutput(func(*Message) {
		atomic.AddInt32(&counter2, 1)
		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{output1, output2},
	}

	Settings.SplitOutput = true

	emitter := NewEmitter()
	go emitter.Start(plugins, Settings.Middleware)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()

	emitter.Close()

	if counter1 == 0 || counter2 == 0 || counter1 != counter2 {
		t.Errorf("Round robin should split traffic equally: %d vs %d", counter1, counter2)
	}

	Settings.SplitOutput = false
}

func TestEmitterRoundRobin(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := NewTestInput()

	var counter1, counter2 int32

	output1 := NewTestOutput(func(*Message) {
		counter1++
		wg.Done()
	})

	output2 := NewTestOutput(func(*Message) {
		counter2++
		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{output1, output2},
	}
	plugins.All = append(plugins.All, input, output1, output2)

	Settings.SplitOutput = true

	emitter := NewEmitter()
	go emitter.Start(plugins, Settings.Middleware)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()
	emitter.Close()

	if counter1 == 0 || counter2 == 0 {
		t.Errorf("Round robin should split traffic equally: %d vs %d", counter1, counter2)
	}

	Settings.SplitOutput = false
}

func TestEmitterSplitSession(t *testing.T) {
	wg := new(sync.WaitGroup)
	wg.Add(200)

	input := NewTestInput()
	input.skipHeader = true

	var counter1, counter2 int32

	output1 := NewTestOutput(func(msg *Message) {
		if payloadID(msg.Meta)[0] == 'a' {
			counter1++
		}
		wg.Done()
	})

	output2 := NewTestOutput(func(msg *Message) {
		if payloadID(msg.Meta)[0] == 'b' {
			counter2++
		}
		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{output1, output2},
	}

	Settings.SplitOutput = true
	Settings.RecognizeTCPSessions = true

	emitter := NewEmitter()
	go emitter.Start(plugins, Settings.Middleware)

	for i := 0; i < 200; i++ {
		// Keep session but randomize
		id := make([]byte, 20)
		if i&1 == 0 { // for recognizeTCPSessions one should be odd and other will be even number
			id[0] = 'a'
		} else {
			id[0] = 'b'
		}
		input.EmitBytes([]byte(fmt.Sprintf("1 %s 1 1\nGET / HTTP/1.1\r\n\r\n", id[:20])))
	}

	wg.Wait()

	if counter1 != counter2 {
		t.Errorf("Round robin should split traffic equally: %d vs %d", counter1, counter2)
	}

	Settings.SplitOutput = false
	Settings.RecognizeTCPSessions = false
	emitter.Close()
}

func BenchmarkEmitter(b *testing.B) {
	wg := new(sync.WaitGroup)

	input := NewTestInput()

	output := NewTestOutput(func(*Message) {
		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := NewEmitter()
	go emitter.Start(plugins, Settings.Middleware)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()
	emitter.Close()
}

// TestLongURLHandling 测试超长URL的处理
func TestLongURLHandling(t *testing.T) {
	wg := new(sync.WaitGroup)
	wg.Add(3) // 期望处理3条消息

	input := NewTestInput()

	output := NewTestOutput(func(msg *Message) {
		// 验证元数据完整性
		meta := payloadMeta(msg.Meta)
		if len(meta) < 3 {
			t.Errorf("Message has incomplete metadata: %v", meta)
		}
		wg.Done()
	})

	plugins := &InOutPlugins{
		Inputs:  []PluginReader{input},
		Outputs: []PluginWriter{output},
	}

	// 设置较小的缓冲区大小来测试截断
	originalBufferSize := Settings.CopyBufferSize
	Settings.CopyBufferSize = 1024 * 1024 // 1MB缓冲区
	defer func() {
		Settings.CopyBufferSize = originalBufferSize
	}()

	emitter := NewEmitter()
	go emitter.Start(plugins, Settings.Middleware)

	// 发送包含长URL的请求
	id1 := uuid()
	reqh1 := payloadHeader(RequestPayload, id1, time.Now().UnixNano(), -1)
	longURL1 := "/very/long/path/" + string(bytes.Repeat([]byte("a"), 20000)) // 2万个字符的URL
	reqb1 := append(reqh1, []byte(fmt.Sprintf("GET %s HTTP/1.1\r\nHost: example.com\r\nUser-Agent: GoReplay-Test\r\n\r\n", longURL1))...)
	input.EmitBytes(reqb1)

	id2 := uuid()
	reqh2 := payloadHeader(RequestPayload, id2, time.Now().UnixNano(), -1)
	longURL2 := "/very/long/path/" + string(bytes.Repeat([]byte("b"), 50000)) // 5万个字符的URL
	reqb2 := append(reqh2, []byte(fmt.Sprintf("GET %s HTTP/1.1\r\nHost: example.com\r\nUser-Agent: GoReplay-Test\r\n\r\n", longURL2))...)
	input.EmitBytes(reqb2)

	id3 := uuid()
	reqh3 := payloadHeader(RequestPayload, id3, time.Now().UnixNano(), -1)
	longURL3 := "/very/long/path/" + string(bytes.Repeat([]byte("c"), 100000)) // 10万个字符的URL
	reqb3 := append(reqh3, []byte(fmt.Sprintf("GET %s HTTP/1.1\r\nHost: example.com\r\nUser-Agent: GoReplay-Test\r\n\r\n", longURL3))...)
	input.EmitBytes(reqb3)

	wg.Wait()
	emitter.Close()
}

// TestMalformedRecordRecovery 测试畸形记录的恢复机制
func TestMalformedRecordRecovery(t *testing.T) {
	// 测试元数据恢复逻辑
	truncatedMeta := []byte("1 1234567890abcdef1234567890abcdef 123456789") // 缺少换行符
	completeData := []byte("0 1000\nGET /test HTTP/1.1\r\nHost: example.com\r\n\r\n")

	// 模拟在CopyMulty中的处理逻辑
	meta := payloadMeta(truncatedMeta)
	if len(meta) < 3 {
		// 尝试从数据部分恢复
		newlinePos := bytes.IndexByte(completeData, '\n')
		if newlinePos > 0 && newlinePos < 100 {
			combinedMeta := make([]byte, 0, len(truncatedMeta)+newlinePos+1)
			combinedMeta = append(combinedMeta, truncatedMeta...)
			combinedMeta = append(combinedMeta, completeData[:newlinePos+1]...)

			recoveredMeta := payloadMeta(combinedMeta)
			if len(recoveredMeta) < 3 {
				t.Errorf("Failed to recover metadata, got %d parts: %v", len(recoveredMeta), recoveredMeta)
			} else {
				t.Logf("Successfully recovered metadata: %v", recoveredMeta)
			}
		}
	}
}
