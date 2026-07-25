package notifier

import "github.com/favonia/cloudflare-ddns/internal/pp"

// Kind identifies the purpose of a notification.
type Kind string

// Notification kinds.
const (
	KindStartup           Kind = "startup"
	KindStartupFailure    Kind = "startup failure"
	KindUpdate            Kind = "update"
	KindUpdateFailure     Kind = "update failure"
	KindSchedulingFailure Kind = "scheduling failure"
	KindCleanup           Kind = "cleanup"
	KindCleanupFailure    Kind = "cleanup failure"
	KindShutdown          Kind = "shutdown"
)

// Notification is a message with metadata describing its purpose.
type Notification struct {
	Kind    Kind
	Message Message
}

// NewNotification creates a notification.
func NewNotification(kind Kind, message Message) Notification {
	return Notification{Kind: kind, Message: message}
}

// NewNotificationf creates a notification with a formatted message.
func NewNotificationf(kind Kind, format string, args ...any) Notification {
	return NewNotification(kind, NewMessagef(format, args...))
}

// Format formats the notification message.
func (n Notification) Format() string { return n.Message.Format() }

// IsEmpty reports whether the notification message is empty.
func (n Notification) IsEmpty() bool { return n.Message.IsEmpty() }

func (n Notification) description(ppfmt pp.PP) string {
	switch n.Kind {
	case KindStartup:
		return "a startup notification"
	case KindStartupFailure:
		return "a startup failure notification"
	case KindUpdate:
		return "an update notification"
	case KindUpdateFailure:
		return "an update failure notification"
	case KindSchedulingFailure:
		return "a scheduling failure notification"
	case KindCleanup:
		return "a cleanup notification"
	case KindCleanupFailure:
		return "a cleanup failure notification"
	case KindShutdown:
		return "a shutdown notification"
	default:
		ppfmt.Noticef(pp.EmojiImpossible,
			"Unknown notification type %q; this should not happen. Please report it at %s",
			n.Kind, pp.IssueReportingURL)
		return "a notification"
	}
}
