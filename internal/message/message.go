package message

type Message struct {
	Ok               bool
	MonitorMessages  []string
	NotifierMessages []string
}

func NewEmpty() Message {
	return Message{
		Ok:               true,
		MonitorMessages:  nil,
		NotifierMessages: nil,
	}
}

func Merge(msgs ...Message) Message {
	var (
		allOk               = true
		allMonitorMessages  = map[bool][]string{true: {}, false: {}}
		allNotifierMessages = []string{}
	)

	for _, msg := range msgs {
		allOk = allOk && msg.Ok
		allMonitorMessages[msg.Ok] = append(allMonitorMessages[msg.Ok], msg.MonitorMessages...)
		allNotifierMessages = append(allNotifierMessages, msg.NotifierMessages...)
	}

	return Message{
		Ok:               allOk,
		MonitorMessages:  allMonitorMessages[allOk],
		NotifierMessages: allNotifierMessages,
	}
}
