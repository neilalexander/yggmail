package yggmail

type Logger interface {
	LogMessage(msg string)
	LogError(errorId int, msg string)
}
