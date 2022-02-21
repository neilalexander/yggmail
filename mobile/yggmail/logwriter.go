package yggmail

import (
	"bytes"
	"io"
	"sync"
)

// An io.Writer for passing logs to the application that integrates yggmail as a mobile library
type LogWriter struct {
	Output io.Writer
	buffer []byte

	lock sync.Mutex

	Logger Logger
}

func (lw *LogWriter) Lock()   { (*lw).lock.Lock() }
func (lw *LogWriter) Unlock() { (*lw).lock.Unlock() }

func (ls *LogWriter) Write(b []byte) (n int, err error) {
	ls.Lock()
	defer ls.Unlock()

	if ls.Logger != nil {
		n = len(b)
		ls.buffer = append(ls.buffer, b...)
		for {
			i := bytes.LastIndexByte(ls.buffer, '\n')
			if i == -1 {
				return
			}
			fullLines := ls.buffer[:i+1]
			ls.Logger.LogMessage(string(fullLines))
			ls.buffer = ls.buffer[i+1:]
		}
	}
	return ls.Output.Write(b)
}
