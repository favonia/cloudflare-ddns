package response

type Response struct {
	Ok               bool
	MonitorMessages  []string
	NotifierMessages []string
}

func Merge(rs ...Response) Response {
	var (
		allOk               = true
		allMonitorMessages  = map[bool][]string{true: {}, false: {}}
		allNotifierMessages = []string{}
	)

	for _, r := range rs {
		allOk = allOk && r.Ok
		allMonitorMessages[r.Ok] = append(allMonitorMessages[r.Ok], r.MonitorMessages...)
		allNotifierMessages = append(allNotifierMessages, r.NotifierMessages...)
	}

	return Response{
		Ok:               allOk,
		MonitorMessages:  allMonitorMessages[allOk],
		NotifierMessages: allNotifierMessages,
	}
}
