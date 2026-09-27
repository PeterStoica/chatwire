package wire

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

type ADVEncryptionType int32

const (
	ADVEncryptionType_E2EE     ADVEncryptionType = 0
	ADVEncryptionType_HOSTED   ADVEncryptionType = 1
	ADVEncryptionType_NON_E2EE ADVEncryptionType = 2
)

var (
	ADVEncryptionType_name = map[int32]string{
		0: "E2EE",
		1: "HOSTED",
		2: "NON_E2EE",
	}
	ADVEncryptionType_value = map[string]int32{
		"E2EE":     0,
		"HOSTED":   1,
		"NON_E2EE": 2,
	}
)

func (x ADVEncryptionType) Enum() *ADVEncryptionType {
	p := new(ADVEncryptionType)
	*p = x
	return p
}

func (x ADVEncryptionType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ADVEncryptionType) Descriptor() protoreflect.EnumDescriptor {
	return file_chatwire_wire_proto_enumTypes[0].Descriptor()
}

func (ADVEncryptionType) Type() protoreflect.EnumType {
	return &file_chatwire_wire_proto_enumTypes[0]
}

func (x ADVEncryptionType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

func (x *ADVEncryptionType) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = ADVEncryptionType(num)
	return nil
}

func (ADVEncryptionType) EnumDescriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{0}
}

type HistorySync_HistorySyncType int32

const (
	HistorySync_INITIAL_BOOTSTRAP HistorySync_HistorySyncType = 0
	HistorySync_INITIAL_STATUS_V3 HistorySync_HistorySyncType = 1
	HistorySync_FULL              HistorySync_HistorySyncType = 2
	HistorySync_RECENT            HistorySync_HistorySyncType = 3
	HistorySync_PUSH_NAME         HistorySync_HistorySyncType = 4
	HistorySync_NON_BLOCKING_DATA HistorySync_HistorySyncType = 5
	HistorySync_ON_DEMAND         HistorySync_HistorySyncType = 6
)

var (
	HistorySync_HistorySyncType_name = map[int32]string{
		0: "INITIAL_BOOTSTRAP",
		1: "INITIAL_STATUS_V3",
		2: "FULL",
		3: "RECENT",
		4: "PUSH_NAME",
		5: "NON_BLOCKING_DATA",
		6: "ON_DEMAND",
	}
	HistorySync_HistorySyncType_value = map[string]int32{
		"INITIAL_BOOTSTRAP": 0,
		"INITIAL_STATUS_V3": 1,
		"FULL":              2,
		"RECENT":            3,
		"PUSH_NAME":         4,
		"NON_BLOCKING_DATA": 5,
		"ON_DEMAND":         6,
	}
)

func (x HistorySync_HistorySyncType) Enum() *HistorySync_HistorySyncType {
	p := new(HistorySync_HistorySyncType)
	*p = x
	return p
}

func (x HistorySync_HistorySyncType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (HistorySync_HistorySyncType) Descriptor() protoreflect.EnumDescriptor {
	return file_chatwire_wire_proto_enumTypes[1].Descriptor()
}

func (HistorySync_HistorySyncType) Type() protoreflect.EnumType {
	return &file_chatwire_wire_proto_enumTypes[1]
}

func (x HistorySync_HistorySyncType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

func (x *HistorySync_HistorySyncType) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = HistorySync_HistorySyncType(num)
	return nil
}

func (HistorySync_HistorySyncType) EnumDescriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{8, 0}
}

type MediaRetryNotification_ResultType int32

const (
	MediaRetryNotification_GENERAL_ERROR    MediaRetryNotification_ResultType = 0
	MediaRetryNotification_SUCCESS          MediaRetryNotification_ResultType = 1
	MediaRetryNotification_NOT_FOUND        MediaRetryNotification_ResultType = 2
	MediaRetryNotification_DECRYPTION_ERROR MediaRetryNotification_ResultType = 3
)

var (
	MediaRetryNotification_ResultType_name = map[int32]string{
		0: "GENERAL_ERROR",
		1: "SUCCESS",
		2: "NOT_FOUND",
		3: "DECRYPTION_ERROR",
	}
	MediaRetryNotification_ResultType_value = map[string]int32{
		"GENERAL_ERROR":    0,
		"SUCCESS":          1,
		"NOT_FOUND":        2,
		"DECRYPTION_ERROR": 3,
	}
)

func (x MediaRetryNotification_ResultType) Enum() *MediaRetryNotification_ResultType {
	p := new(MediaRetryNotification_ResultType)
	*p = x
	return p
}

func (x MediaRetryNotification_ResultType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (MediaRetryNotification_ResultType) Descriptor() protoreflect.EnumDescriptor {
	return file_chatwire_wire_proto_enumTypes[2].Descriptor()
}

func (MediaRetryNotification_ResultType) Type() protoreflect.EnumType {
	return &file_chatwire_wire_proto_enumTypes[2]
}

func (x MediaRetryNotification_ResultType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

func (x *MediaRetryNotification_ResultType) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = MediaRetryNotification_ResultType(num)
	return nil
}

func (MediaRetryNotification_ResultType) EnumDescriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{13, 0}
}

type Message_HistorySyncType int32

const (
	Message_INITIAL_BOOTSTRAP     Message_HistorySyncType = 0
	Message_INITIAL_STATUS_V3     Message_HistorySyncType = 1
	Message_FULL                  Message_HistorySyncType = 2
	Message_RECENT                Message_HistorySyncType = 3
	Message_PUSH_NAME             Message_HistorySyncType = 4
	Message_NON_BLOCKING_DATA     Message_HistorySyncType = 5
	Message_ON_DEMAND             Message_HistorySyncType = 6
	Message_NO_HISTORY            Message_HistorySyncType = 7
	Message_MESSAGE_ACCESS_STATUS Message_HistorySyncType = 8
)

var (
	Message_HistorySyncType_name = map[int32]string{
		0: "INITIAL_BOOTSTRAP",
		1: "INITIAL_STATUS_V3",
		2: "FULL",
		3: "RECENT",
		4: "PUSH_NAME",
		5: "NON_BLOCKING_DATA",
		6: "ON_DEMAND",
		7: "NO_HISTORY",
		8: "MESSAGE_ACCESS_STATUS",
	}
	Message_HistorySyncType_value = map[string]int32{
		"INITIAL_BOOTSTRAP":     0,
		"INITIAL_STATUS_V3":     1,
		"FULL":                  2,
		"RECENT":                3,
		"PUSH_NAME":             4,
		"NON_BLOCKING_DATA":     5,
		"ON_DEMAND":             6,
		"NO_HISTORY":            7,
		"MESSAGE_ACCESS_STATUS": 8,
	}
)

func (x Message_HistorySyncType) Enum() *Message_HistorySyncType {
	p := new(Message_HistorySyncType)
	*p = x
	return p
}

func (x Message_HistorySyncType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Message_HistorySyncType) Descriptor() protoreflect.EnumDescriptor {
	return file_chatwire_wire_proto_enumTypes[3].Descriptor()
}

func (Message_HistorySyncType) Type() protoreflect.EnumType {
	return &file_chatwire_wire_proto_enumTypes[3]
}

func (x Message_HistorySyncType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

func (x *Message_HistorySyncType) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = Message_HistorySyncType(num)
	return nil
}

func (Message_HistorySyncType) EnumDescriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 0}
}

type Message_PollContentType int32

const (
	Message_UNKNOWN Message_PollContentType = 0
	Message_TEXT    Message_PollContentType = 1
	Message_IMAGE   Message_PollContentType = 2
)

var (
	Message_PollContentType_name = map[int32]string{
		0: "UNKNOWN",
		1: "TEXT",
		2: "IMAGE",
	}
	Message_PollContentType_value = map[string]int32{
		"UNKNOWN": 0,
		"TEXT":    1,
		"IMAGE":   2,
	}
)

func (x Message_PollContentType) Enum() *Message_PollContentType {
	p := new(Message_PollContentType)
	*p = x
	return p
}

func (x Message_PollContentType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Message_PollContentType) Descriptor() protoreflect.EnumDescriptor {
	return file_chatwire_wire_proto_enumTypes[4].Descriptor()
}

func (Message_PollContentType) Type() protoreflect.EnumType {
	return &file_chatwire_wire_proto_enumTypes[4]
}

func (x Message_PollContentType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

func (x *Message_PollContentType) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = Message_PollContentType(num)
	return nil
}

func (Message_PollContentType) EnumDescriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 1}
}

type Message_PollType int32

const (
	Message_POLL Message_PollType = 0
	Message_QUIZ Message_PollType = 1
)

var (
	Message_PollType_name = map[int32]string{
		0: "POLL",
		1: "QUIZ",
	}
	Message_PollType_value = map[string]int32{
		"POLL": 0,
		"QUIZ": 1,
	}
)

func (x Message_PollType) Enum() *Message_PollType {
	p := new(Message_PollType)
	*p = x
	return p
}

func (x Message_PollType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Message_PollType) Descriptor() protoreflect.EnumDescriptor {
	return file_chatwire_wire_proto_enumTypes[5].Descriptor()
}

func (Message_PollType) Type() protoreflect.EnumType {
	return &file_chatwire_wire_proto_enumTypes[5]
}

func (x Message_PollType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

func (x *Message_PollType) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = Message_PollType(num)
	return nil
}

func (Message_PollType) EnumDescriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 2}
}

type Message_ProtocolMessage_Type int32

const (
	Message_ProtocolMessage_REVOKE                                       Message_ProtocolMessage_Type = 0
	Message_ProtocolMessage_EPHEMERAL_SETTING                            Message_ProtocolMessage_Type = 3
	Message_ProtocolMessage_EPHEMERAL_SYNC_RESPONSE                      Message_ProtocolMessage_Type = 4
	Message_ProtocolMessage_HISTORY_SYNC_NOTIFICATION                    Message_ProtocolMessage_Type = 5
	Message_ProtocolMessage_APP_STATE_SYNC_KEY_SHARE                     Message_ProtocolMessage_Type = 6
	Message_ProtocolMessage_APP_STATE_SYNC_KEY_REQUEST                   Message_ProtocolMessage_Type = 7
	Message_ProtocolMessage_MSG_FANOUT_BACKFILL_REQUEST                  Message_ProtocolMessage_Type = 8
	Message_ProtocolMessage_INITIAL_SECURITY_NOTIFICATION_SETTING_SYNC   Message_ProtocolMessage_Type = 9
	Message_ProtocolMessage_APP_STATE_FATAL_EXCEPTION_NOTIFICATION       Message_ProtocolMessage_Type = 10
	Message_ProtocolMessage_SHARE_PHONE_NUMBER                           Message_ProtocolMessage_Type = 11
	Message_ProtocolMessage_MESSAGE_EDIT                                 Message_ProtocolMessage_Type = 14
	Message_ProtocolMessage_PEER_DATA_OPERATION_REQUEST_MESSAGE          Message_ProtocolMessage_Type = 16
	Message_ProtocolMessage_PEER_DATA_OPERATION_REQUEST_RESPONSE_MESSAGE Message_ProtocolMessage_Type = 17
	Message_ProtocolMessage_REQUEST_WELCOME_MESSAGE                      Message_ProtocolMessage_Type = 18
	Message_ProtocolMessage_BOT_FEEDBACK_MESSAGE                         Message_ProtocolMessage_Type = 19
	Message_ProtocolMessage_MEDIA_NOTIFY_MESSAGE                         Message_ProtocolMessage_Type = 20
	Message_ProtocolMessage_CLOUD_API_THREAD_CONTROL_NOTIFICATION        Message_ProtocolMessage_Type = 21
	Message_ProtocolMessage_LID_MIGRATION_MAPPING_SYNC                   Message_ProtocolMessage_Type = 22
	Message_ProtocolMessage_REMINDER_MESSAGE                             Message_ProtocolMessage_Type = 23
	Message_ProtocolMessage_BOT_MEMU_ONBOARDING_MESSAGE                  Message_ProtocolMessage_Type = 24
	Message_ProtocolMessage_STATUS_MENTION_MESSAGE                       Message_ProtocolMessage_Type = 25
	Message_ProtocolMessage_STOP_GENERATION_MESSAGE                      Message_ProtocolMessage_Type = 26
	Message_ProtocolMessage_LIMIT_SHARING                                Message_ProtocolMessage_Type = 27
	Message_ProtocolMessage_AI_PSI_METADATA                              Message_ProtocolMessage_Type = 28
	Message_ProtocolMessage_AI_QUERY_FANOUT                              Message_ProtocolMessage_Type = 29
	Message_ProtocolMessage_GROUP_MEMBER_LABEL_CHANGE                    Message_ProtocolMessage_Type = 30
	Message_ProtocolMessage_AI_MEDIA_COLLECTION_MESSAGE                  Message_ProtocolMessage_Type = 31
	Message_ProtocolMessage_MESSAGE_UNSCHEDULE                           Message_ProtocolMessage_Type = 32
	Message_ProtocolMessage_CHAT_THEME_SETTING                           Message_ProtocolMessage_Type = 34
	Message_ProtocolMessage_AI_METADATA_OPERATION                        Message_ProtocolMessage_Type = 35
	Message_ProtocolMessage_MARK_AS_VERIFIED_ACTION                      Message_ProtocolMessage_Type = 36
	Message_ProtocolMessage_COEX_STATE_SYNC                              Message_ProtocolMessage_Type = 37
	Message_ProtocolMessage_ACP2_SETTING                                 Message_ProtocolMessage_Type = 39
	Message_ProtocolMessage_SHARED_DEVICE_CONTACT_HASH_KEY_SHARE         Message_ProtocolMessage_Type = 40
	Message_ProtocolMessage_SHARED_DEVICE_CONTACT_HASH_KEY_REQUEST       Message_ProtocolMessage_Type = 41
)

var (
	Message_ProtocolMessage_Type_name = map[int32]string{
		0:  "REVOKE",
		3:  "EPHEMERAL_SETTING",
		4:  "EPHEMERAL_SYNC_RESPONSE",
		5:  "HISTORY_SYNC_NOTIFICATION",
		6:  "APP_STATE_SYNC_KEY_SHARE",
		7:  "APP_STATE_SYNC_KEY_REQUEST",
		8:  "MSG_FANOUT_BACKFILL_REQUEST",
		9:  "INITIAL_SECURITY_NOTIFICATION_SETTING_SYNC",
		10: "APP_STATE_FATAL_EXCEPTION_NOTIFICATION",
		11: "SHARE_PHONE_NUMBER",
		14: "MESSAGE_EDIT",
		16: "PEER_DATA_OPERATION_REQUEST_MESSAGE",
		17: "PEER_DATA_OPERATION_REQUEST_RESPONSE_MESSAGE",
		18: "REQUEST_WELCOME_MESSAGE",
		19: "BOT_FEEDBACK_MESSAGE",
		20: "MEDIA_NOTIFY_MESSAGE",
		21: "CLOUD_API_THREAD_CONTROL_NOTIFICATION",
		22: "LID_MIGRATION_MAPPING_SYNC",
		23: "REMINDER_MESSAGE",
		24: "BOT_MEMU_ONBOARDING_MESSAGE",
		25: "STATUS_MENTION_MESSAGE",
		26: "STOP_GENERATION_MESSAGE",
		27: "LIMIT_SHARING",
		28: "AI_PSI_METADATA",
		29: "AI_QUERY_FANOUT",
		30: "GROUP_MEMBER_LABEL_CHANGE",
		31: "AI_MEDIA_COLLECTION_MESSAGE",
		32: "MESSAGE_UNSCHEDULE",
		34: "CHAT_THEME_SETTING",
		35: "AI_METADATA_OPERATION",
		36: "MARK_AS_VERIFIED_ACTION",
		37: "COEX_STATE_SYNC",
		39: "ACP2_SETTING",
		40: "SHARED_DEVICE_CONTACT_HASH_KEY_SHARE",
		41: "SHARED_DEVICE_CONTACT_HASH_KEY_REQUEST",
	}
	Message_ProtocolMessage_Type_value = map[string]int32{
		"REVOKE":                                       0,
		"EPHEMERAL_SETTING":                            3,
		"EPHEMERAL_SYNC_RESPONSE":                      4,
		"HISTORY_SYNC_NOTIFICATION":                    5,
		"APP_STATE_SYNC_KEY_SHARE":                     6,
		"APP_STATE_SYNC_KEY_REQUEST":                   7,
		"MSG_FANOUT_BACKFILL_REQUEST":                  8,
		"INITIAL_SECURITY_NOTIFICATION_SETTING_SYNC":   9,
		"APP_STATE_FATAL_EXCEPTION_NOTIFICATION":       10,
		"SHARE_PHONE_NUMBER":                           11,
		"MESSAGE_EDIT":                                 14,
		"PEER_DATA_OPERATION_REQUEST_MESSAGE":          16,
		"PEER_DATA_OPERATION_REQUEST_RESPONSE_MESSAGE": 17,
		"REQUEST_WELCOME_MESSAGE":                      18,
		"BOT_FEEDBACK_MESSAGE":                         19,
		"MEDIA_NOTIFY_MESSAGE":                         20,
		"CLOUD_API_THREAD_CONTROL_NOTIFICATION":        21,
		"LID_MIGRATION_MAPPING_SYNC":                   22,
		"REMINDER_MESSAGE":                             23,
		"BOT_MEMU_ONBOARDING_MESSAGE":                  24,
		"STATUS_MENTION_MESSAGE":                       25,
		"STOP_GENERATION_MESSAGE":                      26,
		"LIMIT_SHARING":                                27,
		"AI_PSI_METADATA":                              28,
		"AI_QUERY_FANOUT":                              29,
		"GROUP_MEMBER_LABEL_CHANGE":                    30,
		"AI_MEDIA_COLLECTION_MESSAGE":                  31,
		"MESSAGE_UNSCHEDULE":                           32,
		"CHAT_THEME_SETTING":                           34,
		"AI_METADATA_OPERATION":                        35,
		"MARK_AS_VERIFIED_ACTION":                      36,
		"COEX_STATE_SYNC":                              37,
		"ACP2_SETTING":                                 39,
		"SHARED_DEVICE_CONTACT_HASH_KEY_SHARE":         40,
		"SHARED_DEVICE_CONTACT_HASH_KEY_REQUEST":       41,
	}
)

func (x Message_ProtocolMessage_Type) Enum() *Message_ProtocolMessage_Type {
	p := new(Message_ProtocolMessage_Type)
	*p = x
	return p
}

func (x Message_ProtocolMessage_Type) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Message_ProtocolMessage_Type) Descriptor() protoreflect.EnumDescriptor {
	return file_chatwire_wire_proto_enumTypes[6].Descriptor()
}

func (Message_ProtocolMessage_Type) Type() protoreflect.EnumType {
	return &file_chatwire_wire_proto_enumTypes[6]
}

func (x Message_ProtocolMessage_Type) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

func (x *Message_ProtocolMessage_Type) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = Message_ProtocolMessage_Type(num)
	return nil
}

func (Message_ProtocolMessage_Type) EnumDescriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 43, 0}
}

type Message_SecretEncryptedMessage_SecretEncType int32

const (
	Message_SecretEncryptedMessage_UNKNOWN          Message_SecretEncryptedMessage_SecretEncType = 0
	Message_SecretEncryptedMessage_EVENT_EDIT       Message_SecretEncryptedMessage_SecretEncType = 1
	Message_SecretEncryptedMessage_MESSAGE_EDIT     Message_SecretEncryptedMessage_SecretEncType = 2
	Message_SecretEncryptedMessage_MESSAGE_SCHEDULE Message_SecretEncryptedMessage_SecretEncType = 3
	Message_SecretEncryptedMessage_POLL_EDIT        Message_SecretEncryptedMessage_SecretEncType = 4
	Message_SecretEncryptedMessage_POLL_ADD_OPTION  Message_SecretEncryptedMessage_SecretEncType = 5
)

var (
	Message_SecretEncryptedMessage_SecretEncType_name = map[int32]string{
		0: "UNKNOWN",
		1: "EVENT_EDIT",
		2: "MESSAGE_EDIT",
		3: "MESSAGE_SCHEDULE",
		4: "POLL_EDIT",
		5: "POLL_ADD_OPTION",
	}
	Message_SecretEncryptedMessage_SecretEncType_value = map[string]int32{
		"UNKNOWN":          0,
		"EVENT_EDIT":       1,
		"MESSAGE_EDIT":     2,
		"MESSAGE_SCHEDULE": 3,
		"POLL_EDIT":        4,
		"POLL_ADD_OPTION":  5,
	}
)

func (x Message_SecretEncryptedMessage_SecretEncType) Enum() *Message_SecretEncryptedMessage_SecretEncType {
	p := new(Message_SecretEncryptedMessage_SecretEncType)
	*p = x
	return p
}

func (x Message_SecretEncryptedMessage_SecretEncType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Message_SecretEncryptedMessage_SecretEncType) Descriptor() protoreflect.EnumDescriptor {
	return file_chatwire_wire_proto_enumTypes[7].Descriptor()
}

func (Message_SecretEncryptedMessage_SecretEncType) Type() protoreflect.EnumType {
	return &file_chatwire_wire_proto_enumTypes[7]
}

func (x Message_SecretEncryptedMessage_SecretEncType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

func (x *Message_SecretEncryptedMessage_SecretEncType) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = Message_SecretEncryptedMessage_SecretEncType(num)
	return nil
}

func (Message_SecretEncryptedMessage_SecretEncType) EnumDescriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 46, 0}
}

type MessageAddOn_MessageAddOnType int32

const (
	MessageAddOn_UNDEFINED      MessageAddOn_MessageAddOnType = 0
	MessageAddOn_REACTION       MessageAddOn_MessageAddOnType = 1
	MessageAddOn_EVENT_RESPONSE MessageAddOn_MessageAddOnType = 2
	MessageAddOn_POLL_UPDATE    MessageAddOn_MessageAddOnType = 3
	MessageAddOn_PIN_IN_CHAT    MessageAddOn_MessageAddOnType = 4
)

var (
	MessageAddOn_MessageAddOnType_name = map[int32]string{
		0: "UNDEFINED",
		1: "REACTION",
		2: "EVENT_RESPONSE",
		3: "POLL_UPDATE",
		4: "PIN_IN_CHAT",
	}
	MessageAddOn_MessageAddOnType_value = map[string]int32{
		"UNDEFINED":      0,
		"REACTION":       1,
		"EVENT_RESPONSE": 2,
		"POLL_UPDATE":    3,
		"PIN_IN_CHAT":    4,
	}
)

func (x MessageAddOn_MessageAddOnType) Enum() *MessageAddOn_MessageAddOnType {
	p := new(MessageAddOn_MessageAddOnType)
	*p = x
	return p
}

func (x MessageAddOn_MessageAddOnType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (MessageAddOn_MessageAddOnType) Descriptor() protoreflect.EnumDescriptor {
	return file_chatwire_wire_proto_enumTypes[8].Descriptor()
}

func (MessageAddOn_MessageAddOnType) Type() protoreflect.EnumType {
	return &file_chatwire_wire_proto_enumTypes[8]
}

func (x MessageAddOn_MessageAddOnType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

func (x *MessageAddOn_MessageAddOnType) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = MessageAddOn_MessageAddOnType(num)
	return nil
}

func (MessageAddOn_MessageAddOnType) EnumDescriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{15, 0}
}

type SyncdMutation_SyncdOperation int32

const (
	SyncdMutation_SET    SyncdMutation_SyncdOperation = 0
	SyncdMutation_REMOVE SyncdMutation_SyncdOperation = 1
)

var (
	SyncdMutation_SyncdOperation_name = map[int32]string{
		0: "SET",
		1: "REMOVE",
	}
	SyncdMutation_SyncdOperation_value = map[string]int32{
		"SET":    0,
		"REMOVE": 1,
	}
)

func (x SyncdMutation_SyncdOperation) Enum() *SyncdMutation_SyncdOperation {
	p := new(SyncdMutation_SyncdOperation)
	*p = x
	return p
}

func (x SyncdMutation_SyncdOperation) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (SyncdMutation_SyncdOperation) Descriptor() protoreflect.EnumDescriptor {
	return file_chatwire_wire_proto_enumTypes[9].Descriptor()
}

func (SyncdMutation_SyncdOperation) Type() protoreflect.EnumType {
	return &file_chatwire_wire_proto_enumTypes[9]
}

func (x SyncdMutation_SyncdOperation) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

func (x *SyncdMutation_SyncdOperation) UnmarshalJSON(b []byte) error {
	num, err := protoimpl.X.UnmarshalJSONEnum(x.Descriptor(), b)
	if err != nil {
		return err
	}
	*x = SyncdMutation_SyncdOperation(num)
	return nil
}

func (SyncdMutation_SyncdOperation) EnumDescriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{31, 0}
}

type ADVDeviceIdentity struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RawId         *uint32                `protobuf:"varint,1,opt,name=rawId" json:"rawId,omitempty"`
	Timestamp     *uint64                `protobuf:"varint,2,opt,name=timestamp" json:"timestamp,omitempty"`
	KeyIndex      *uint32                `protobuf:"varint,3,opt,name=keyIndex" json:"keyIndex,omitempty"`
	AccountType   *ADVEncryptionType     `protobuf:"varint,4,opt,name=accountType,enum=chatwire.wire.ADVEncryptionType,def=0" json:"accountType,omitempty"`
	DeviceType    *ADVEncryptionType     `protobuf:"varint,5,opt,name=deviceType,enum=chatwire.wire.ADVEncryptionType,def=0" json:"deviceType,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

const (
	Default_ADVDeviceIdentity_AccountType = ADVEncryptionType_E2EE
	Default_ADVDeviceIdentity_DeviceType  = ADVEncryptionType_E2EE
)

func (x *ADVDeviceIdentity) Reset() {
	*x = ADVDeviceIdentity{}
	mi := &file_chatwire_wire_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ADVDeviceIdentity) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ADVDeviceIdentity) ProtoMessage() {}

func (x *ADVDeviceIdentity) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ADVDeviceIdentity) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{0}
}

func (x *ADVDeviceIdentity) GetRawId() uint32 {
	if x != nil && x.RawId != nil {
		return *x.RawId
	}
	return 0
}

func (x *ADVDeviceIdentity) GetTimestamp() uint64 {
	if x != nil && x.Timestamp != nil {
		return *x.Timestamp
	}
	return 0
}

func (x *ADVDeviceIdentity) GetKeyIndex() uint32 {
	if x != nil && x.KeyIndex != nil {
		return *x.KeyIndex
	}
	return 0
}

func (x *ADVDeviceIdentity) GetAccountType() ADVEncryptionType {
	if x != nil && x.AccountType != nil {
		return *x.AccountType
	}
	return Default_ADVDeviceIdentity_AccountType
}

func (x *ADVDeviceIdentity) GetDeviceType() ADVEncryptionType {
	if x != nil && x.DeviceType != nil {
		return *x.DeviceType
	}
	return Default_ADVDeviceIdentity_DeviceType
}

type ADVSignedDeviceIdentity struct {
	state               protoimpl.MessageState `protogen:"open.v1"`
	Details             []byte                 `protobuf:"bytes,1,opt,name=details" json:"details,omitempty"`
	AccountSignatureKey []byte                 `protobuf:"bytes,2,opt,name=accountSignatureKey" json:"accountSignatureKey,omitempty"`
	AccountSignature    []byte                 `protobuf:"bytes,3,opt,name=accountSignature" json:"accountSignature,omitempty"`
	DeviceSignature     []byte                 `protobuf:"bytes,4,opt,name=deviceSignature" json:"deviceSignature,omitempty"`
	unknownFields       protoimpl.UnknownFields
	sizeCache           protoimpl.SizeCache
}

func (x *ADVSignedDeviceIdentity) Reset() {
	*x = ADVSignedDeviceIdentity{}
	mi := &file_chatwire_wire_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ADVSignedDeviceIdentity) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ADVSignedDeviceIdentity) ProtoMessage() {}

func (x *ADVSignedDeviceIdentity) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ADVSignedDeviceIdentity) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{1}
}

func (x *ADVSignedDeviceIdentity) GetDetails() []byte {
	if x != nil {
		return x.Details
	}
	return nil
}

func (x *ADVSignedDeviceIdentity) GetAccountSignatureKey() []byte {
	if x != nil {
		return x.AccountSignatureKey
	}
	return nil
}

func (x *ADVSignedDeviceIdentity) GetAccountSignature() []byte {
	if x != nil {
		return x.AccountSignature
	}
	return nil
}

func (x *ADVSignedDeviceIdentity) GetDeviceSignature() []byte {
	if x != nil {
		return x.DeviceSignature
	}
	return nil
}

type ADVSignedDeviceIdentityHMAC struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Details       []byte                 `protobuf:"bytes,1,opt,name=details" json:"details,omitempty"`
	Hmac          []byte                 `protobuf:"bytes,2,opt,name=hmac" json:"hmac,omitempty"`
	AccountType   *ADVEncryptionType     `protobuf:"varint,3,opt,name=accountType,enum=chatwire.wire.ADVEncryptionType,def=0" json:"accountType,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

const (
	Default_ADVSignedDeviceIdentityHMAC_AccountType = ADVEncryptionType_E2EE
)

func (x *ADVSignedDeviceIdentityHMAC) Reset() {
	*x = ADVSignedDeviceIdentityHMAC{}
	mi := &file_chatwire_wire_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ADVSignedDeviceIdentityHMAC) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ADVSignedDeviceIdentityHMAC) ProtoMessage() {}

func (x *ADVSignedDeviceIdentityHMAC) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ADVSignedDeviceIdentityHMAC) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{2}
}

func (x *ADVSignedDeviceIdentityHMAC) GetDetails() []byte {
	if x != nil {
		return x.Details
	}
	return nil
}

func (x *ADVSignedDeviceIdentityHMAC) GetHmac() []byte {
	if x != nil {
		return x.Hmac
	}
	return nil
}

func (x *ADVSignedDeviceIdentityHMAC) GetAccountType() ADVEncryptionType {
	if x != nil && x.AccountType != nil {
		return *x.AccountType
	}
	return Default_ADVSignedDeviceIdentityHMAC_AccountType
}

type AIRichResponseMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,4,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *AIRichResponseMessage) Reset() {
	*x = AIRichResponseMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AIRichResponseMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AIRichResponseMessage) ProtoMessage() {}

func (x *AIRichResponseMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*AIRichResponseMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{3}
}

func (x *AIRichResponseMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type ClientPayload struct {
	state             protoimpl.MessageState                       `protogen:"open.v1"`
	Username          *uint64                                      `protobuf:"varint,1,opt,name=username" json:"username,omitempty"`
	Passive           *bool                                        `protobuf:"varint,3,opt,name=passive" json:"passive,omitempty"`
	Device            *uint32                                      `protobuf:"varint,18,opt,name=device" json:"device,omitempty"`
	DevicePairingData *ClientPayload_DevicePairingRegistrationData `protobuf:"bytes,19,opt,name=devicePairingData" json:"devicePairingData,omitempty"`
	Pull              *bool                                        `protobuf:"varint,33,opt,name=pull" json:"pull,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *ClientPayload) Reset() {
	*x = ClientPayload{}
	mi := &file_chatwire_wire_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClientPayload) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClientPayload) ProtoMessage() {}

func (x *ClientPayload) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ClientPayload) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{4}
}

func (x *ClientPayload) GetUsername() uint64 {
	if x != nil && x.Username != nil {
		return *x.Username
	}
	return 0
}

func (x *ClientPayload) GetPassive() bool {
	if x != nil && x.Passive != nil {
		return *x.Passive
	}
	return false
}

func (x *ClientPayload) GetDevice() uint32 {
	if x != nil && x.Device != nil {
		return *x.Device
	}
	return 0
}

func (x *ClientPayload) GetDevicePairingData() *ClientPayload_DevicePairingRegistrationData {
	if x != nil {
		return x.DevicePairingData
	}
	return nil
}

func (x *ClientPayload) GetPull() bool {
	if x != nil && x.Pull != nil {
		return *x.Pull
	}
	return false
}

type ContextInfo struct {
	state           protoimpl.MessageState `protogen:"open.v1"`
	StanzaId        *string                `protobuf:"bytes,1,opt,name=stanzaId" json:"stanzaId,omitempty"`
	Participant     *string                `protobuf:"bytes,2,opt,name=participant" json:"participant,omitempty"`
	QuotedMessage   *Message               `protobuf:"bytes,3,opt,name=quotedMessage" json:"quotedMessage,omitempty"`
	RemoteJid       *string                `protobuf:"bytes,4,opt,name=remoteJid" json:"remoteJid,omitempty"`
	MentionedJid    []string               `protobuf:"bytes,15,rep,name=mentionedJid" json:"mentionedJid,omitempty"`
	ForwardingScore *uint32                `protobuf:"varint,21,opt,name=forwardingScore" json:"forwardingScore,omitempty"`
	IsForwarded     *bool                  `protobuf:"varint,22,opt,name=isForwarded" json:"isForwarded,omitempty"`
	unknownFields   protoimpl.UnknownFields
	sizeCache       protoimpl.SizeCache
}

func (x *ContextInfo) Reset() {
	*x = ContextInfo{}
	mi := &file_chatwire_wire_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ContextInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ContextInfo) ProtoMessage() {}

func (x *ContextInfo) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ContextInfo) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{5}
}

func (x *ContextInfo) GetStanzaId() string {
	if x != nil && x.StanzaId != nil {
		return *x.StanzaId
	}
	return ""
}

func (x *ContextInfo) GetParticipant() string {
	if x != nil && x.Participant != nil {
		return *x.Participant
	}
	return ""
}

func (x *ContextInfo) GetQuotedMessage() *Message {
	if x != nil {
		return x.QuotedMessage
	}
	return nil
}

func (x *ContextInfo) GetRemoteJid() string {
	if x != nil && x.RemoteJid != nil {
		return *x.RemoteJid
	}
	return ""
}

func (x *ContextInfo) GetMentionedJid() []string {
	if x != nil {
		return x.MentionedJid
	}
	return nil
}

func (x *ContextInfo) GetForwardingScore() uint32 {
	if x != nil && x.ForwardingScore != nil {
		return *x.ForwardingScore
	}
	return 0
}

func (x *ContextInfo) GetIsForwarded() bool {
	if x != nil && x.IsForwarded != nil {
		return *x.IsForwarded
	}
	return false
}

type Conversation struct {
	state                 protoimpl.MessageState `protogen:"open.v1"`
	Id                    *string                `protobuf:"bytes,1,req,name=id" json:"id,omitempty"`
	Messages              []*HistorySyncMsg      `protobuf:"bytes,2,rep,name=messages" json:"messages,omitempty"`
	LastMsgTimestamp      *uint64                `protobuf:"varint,5,opt,name=lastMsgTimestamp" json:"lastMsgTimestamp,omitempty"`
	UnreadCount           *uint32                `protobuf:"varint,6,opt,name=unreadCount" json:"unreadCount,omitempty"`
	ReadOnly              *bool                  `protobuf:"varint,7,opt,name=readOnly" json:"readOnly,omitempty"`
	ConversationTimestamp *uint64                `protobuf:"varint,12,opt,name=conversationTimestamp" json:"conversationTimestamp,omitempty"`
	Name                  *string                `protobuf:"bytes,13,opt,name=name" json:"name,omitempty"`
	Archived              *bool                  `protobuf:"varint,16,opt,name=archived" json:"archived,omitempty"`
	Pinned                *uint32                `protobuf:"varint,24,opt,name=pinned" json:"pinned,omitempty"`
	MuteEndTime           *uint64                `protobuf:"varint,25,opt,name=muteEndTime" json:"muteEndTime,omitempty"`
	DisplayName           *string                `protobuf:"bytes,38,opt,name=displayName" json:"displayName,omitempty"`
	unknownFields         protoimpl.UnknownFields
	sizeCache             protoimpl.SizeCache
}

func (x *Conversation) Reset() {
	*x = Conversation{}
	mi := &file_chatwire_wire_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Conversation) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Conversation) ProtoMessage() {}

func (x *Conversation) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Conversation) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{6}
}

func (x *Conversation) GetId() string {
	if x != nil && x.Id != nil {
		return *x.Id
	}
	return ""
}

func (x *Conversation) GetMessages() []*HistorySyncMsg {
	if x != nil {
		return x.Messages
	}
	return nil
}

func (x *Conversation) GetLastMsgTimestamp() uint64 {
	if x != nil && x.LastMsgTimestamp != nil {
		return *x.LastMsgTimestamp
	}
	return 0
}

func (x *Conversation) GetUnreadCount() uint32 {
	if x != nil && x.UnreadCount != nil {
		return *x.UnreadCount
	}
	return 0
}

func (x *Conversation) GetReadOnly() bool {
	if x != nil && x.ReadOnly != nil {
		return *x.ReadOnly
	}
	return false
}

func (x *Conversation) GetConversationTimestamp() uint64 {
	if x != nil && x.ConversationTimestamp != nil {
		return *x.ConversationTimestamp
	}
	return 0
}

func (x *Conversation) GetName() string {
	if x != nil && x.Name != nil {
		return *x.Name
	}
	return ""
}

func (x *Conversation) GetArchived() bool {
	if x != nil && x.Archived != nil {
		return *x.Archived
	}
	return false
}

func (x *Conversation) GetPinned() uint32 {
	if x != nil && x.Pinned != nil {
		return *x.Pinned
	}
	return 0
}

func (x *Conversation) GetMuteEndTime() uint64 {
	if x != nil && x.MuteEndTime != nil {
		return *x.MuteEndTime
	}
	return 0
}

func (x *Conversation) GetDisplayName() string {
	if x != nil && x.DisplayName != nil {
		return *x.DisplayName
	}
	return ""
}

type ExternalBlobReference struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	MediaKey      []byte                 `protobuf:"bytes,1,opt,name=mediaKey" json:"mediaKey,omitempty"`
	DirectPath    *string                `protobuf:"bytes,2,opt,name=directPath" json:"directPath,omitempty"`
	FileSha256    []byte                 `protobuf:"bytes,5,opt,name=fileSha256" json:"fileSha256,omitempty"`
	FileEncSha256 []byte                 `protobuf:"bytes,6,opt,name=fileEncSha256" json:"fileEncSha256,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ExternalBlobReference) Reset() {
	*x = ExternalBlobReference{}
	mi := &file_chatwire_wire_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ExternalBlobReference) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExternalBlobReference) ProtoMessage() {}

func (x *ExternalBlobReference) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ExternalBlobReference) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{7}
}

func (x *ExternalBlobReference) GetMediaKey() []byte {
	if x != nil {
		return x.MediaKey
	}
	return nil
}

func (x *ExternalBlobReference) GetDirectPath() string {
	if x != nil && x.DirectPath != nil {
		return *x.DirectPath
	}
	return ""
}

func (x *ExternalBlobReference) GetFileSha256() []byte {
	if x != nil {
		return x.FileSha256
	}
	return nil
}

func (x *ExternalBlobReference) GetFileEncSha256() []byte {
	if x != nil {
		return x.FileEncSha256
	}
	return nil
}

type HistorySync struct {
	state                    protoimpl.MessageState       `protogen:"open.v1"`
	SyncType                 *HistorySync_HistorySyncType `protobuf:"varint,1,req,name=syncType,enum=chatwire.wire.HistorySync_HistorySyncType" json:"syncType,omitempty"`
	Conversations            []*Conversation              `protobuf:"bytes,2,rep,name=conversations" json:"conversations,omitempty"`
	ChunkOrder               *uint32                      `protobuf:"varint,5,opt,name=chunkOrder" json:"chunkOrder,omitempty"`
	Progress                 *uint32                      `protobuf:"varint,6,opt,name=progress" json:"progress,omitempty"`
	Pushnames                []*Pushname                  `protobuf:"bytes,7,rep,name=pushnames" json:"pushnames,omitempty"`
	PhoneNumberToLidMappings []*PhoneNumberToLIDMapping   `protobuf:"bytes,15,rep,name=phoneNumberToLidMappings" json:"phoneNumberToLidMappings,omitempty"`
	InlineContacts           []*InlineContact             `protobuf:"bytes,20,rep,name=inlineContacts" json:"inlineContacts,omitempty"`
	unknownFields            protoimpl.UnknownFields
	sizeCache                protoimpl.SizeCache
}

func (x *HistorySync) Reset() {
	*x = HistorySync{}
	mi := &file_chatwire_wire_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HistorySync) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HistorySync) ProtoMessage() {}

func (x *HistorySync) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*HistorySync) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{8}
}

func (x *HistorySync) GetSyncType() HistorySync_HistorySyncType {
	if x != nil && x.SyncType != nil {
		return *x.SyncType
	}
	return HistorySync_INITIAL_BOOTSTRAP
}

func (x *HistorySync) GetConversations() []*Conversation {
	if x != nil {
		return x.Conversations
	}
	return nil
}

func (x *HistorySync) GetChunkOrder() uint32 {
	if x != nil && x.ChunkOrder != nil {
		return *x.ChunkOrder
	}
	return 0
}

func (x *HistorySync) GetProgress() uint32 {
	if x != nil && x.Progress != nil {
		return *x.Progress
	}
	return 0
}

func (x *HistorySync) GetPushnames() []*Pushname {
	if x != nil {
		return x.Pushnames
	}
	return nil
}

func (x *HistorySync) GetPhoneNumberToLidMappings() []*PhoneNumberToLIDMapping {
	if x != nil {
		return x.PhoneNumberToLidMappings
	}
	return nil
}

func (x *HistorySync) GetInlineContacts() []*InlineContact {
	if x != nil {
		return x.InlineContacts
	}
	return nil
}

type HistorySyncMsg struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Message       *MessageInfo           `protobuf:"bytes,1,opt,name=message" json:"message,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HistorySyncMsg) Reset() {
	*x = HistorySyncMsg{}
	mi := &file_chatwire_wire_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HistorySyncMsg) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HistorySyncMsg) ProtoMessage() {}

func (x *HistorySyncMsg) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*HistorySyncMsg) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{9}
}

func (x *HistorySyncMsg) GetMessage() *MessageInfo {
	if x != nil {
		return x.Message
	}
	return nil
}

type InlineContact struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	PnJid         *string                `protobuf:"bytes,1,opt,name=pnJid" json:"pnJid,omitempty"`
	LidJid        *string                `protobuf:"bytes,2,opt,name=lidJid" json:"lidJid,omitempty"`
	FullName      *string                `protobuf:"bytes,3,opt,name=fullName" json:"fullName,omitempty"`
	FirstName     *string                `protobuf:"bytes,4,opt,name=firstName" json:"firstName,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *InlineContact) Reset() {
	*x = InlineContact{}
	mi := &file_chatwire_wire_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *InlineContact) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InlineContact) ProtoMessage() {}

func (x *InlineContact) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*InlineContact) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{10}
}

func (x *InlineContact) GetPnJid() string {
	if x != nil && x.PnJid != nil {
		return *x.PnJid
	}
	return ""
}

func (x *InlineContact) GetLidJid() string {
	if x != nil && x.LidJid != nil {
		return *x.LidJid
	}
	return ""
}

func (x *InlineContact) GetFullName() string {
	if x != nil && x.FullName != nil {
		return *x.FullName
	}
	return ""
}

func (x *InlineContact) GetFirstName() string {
	if x != nil && x.FirstName != nil {
		return *x.FirstName
	}
	return ""
}

type KeyId struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            []byte                 `protobuf:"bytes,1,opt,name=id" json:"id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *KeyId) Reset() {
	*x = KeyId{}
	mi := &file_chatwire_wire_proto_msgTypes[11]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *KeyId) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*KeyId) ProtoMessage() {}

func (x *KeyId) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[11]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*KeyId) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{11}
}

func (x *KeyId) GetId() []byte {
	if x != nil {
		return x.Id
	}
	return nil
}

type LegacyMessage struct {
	state         protoimpl.MessageState   `protogen:"open.v1"`
	PollVote      *Message_PollVoteMessage `protobuf:"bytes,2,opt,name=pollVote" json:"pollVote,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *LegacyMessage) Reset() {
	*x = LegacyMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[12]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *LegacyMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LegacyMessage) ProtoMessage() {}

func (x *LegacyMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[12]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*LegacyMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{12}
}

func (x *LegacyMessage) GetPollVote() *Message_PollVoteMessage {
	if x != nil {
		return x.PollVote
	}
	return nil
}

type MediaRetryNotification struct {
	state         protoimpl.MessageState             `protogen:"open.v1"`
	StanzaId      *string                            `protobuf:"bytes,1,opt,name=stanzaId" json:"stanzaId,omitempty"`
	DirectPath    *string                            `protobuf:"bytes,2,opt,name=directPath" json:"directPath,omitempty"`
	Result        *MediaRetryNotification_ResultType `protobuf:"varint,3,opt,name=result,enum=chatwire.wire.MediaRetryNotification_ResultType" json:"result,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *MediaRetryNotification) Reset() {
	*x = MediaRetryNotification{}
	mi := &file_chatwire_wire_proto_msgTypes[13]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MediaRetryNotification) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MediaRetryNotification) ProtoMessage() {}

func (x *MediaRetryNotification) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[13]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*MediaRetryNotification) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{13}
}

func (x *MediaRetryNotification) GetStanzaId() string {
	if x != nil && x.StanzaId != nil {
		return *x.StanzaId
	}
	return ""
}

func (x *MediaRetryNotification) GetDirectPath() string {
	if x != nil && x.DirectPath != nil {
		return *x.DirectPath
	}
	return ""
}

func (x *MediaRetryNotification) GetResult() MediaRetryNotification_ResultType {
	if x != nil && x.Result != nil {
		return *x.Result
	}
	return MediaRetryNotification_GENERAL_ERROR
}

type Message struct {
	state                             protoimpl.MessageState                   `protogen:"open.v1"`
	Conversation                      *string                                  `protobuf:"bytes,1,opt,name=conversation" json:"conversation,omitempty"`
	SenderKeyDistributionMessage      *Message_SenderKeyDistributionMessage    `protobuf:"bytes,2,opt,name=senderKeyDistributionMessage" json:"senderKeyDistributionMessage,omitempty"`
	ImageMessage                      *Message_ImageMessage                    `protobuf:"bytes,3,opt,name=imageMessage" json:"imageMessage,omitempty"`
	ContactMessage                    *Message_ContactMessage                  `protobuf:"bytes,4,opt,name=contactMessage" json:"contactMessage,omitempty"`
	LocationMessage                   *Message_LocationMessage                 `protobuf:"bytes,5,opt,name=locationMessage" json:"locationMessage,omitempty"`
	ExtendedTextMessage               *Message_ExtendedTextMessage             `protobuf:"bytes,6,opt,name=extendedTextMessage" json:"extendedTextMessage,omitempty"`
	DocumentMessage                   *Message_DocumentMessage                 `protobuf:"bytes,7,opt,name=documentMessage" json:"documentMessage,omitempty"`
	AudioMessage                      *Message_AudioMessage                    `protobuf:"bytes,8,opt,name=audioMessage" json:"audioMessage,omitempty"`
	VideoMessage                      *Message_VideoMessage                    `protobuf:"bytes,9,opt,name=videoMessage" json:"videoMessage,omitempty"`
	Call                              *Message_Call                            `protobuf:"bytes,10,opt,name=call" json:"call,omitempty"`
	ProtocolMessage                   *Message_ProtocolMessage                 `protobuf:"bytes,12,opt,name=protocolMessage" json:"protocolMessage,omitempty"`
	ContactsArrayMessage              *Message_ContactsArrayMessage            `protobuf:"bytes,13,opt,name=contactsArrayMessage" json:"contactsArrayMessage,omitempty"`
	LiveLocationMessage               *Message_LiveLocationMessage             `protobuf:"bytes,18,opt,name=liveLocationMessage" json:"liveLocationMessage,omitempty"`
	TemplateMessage                   *Message_TemplateMessage                 `protobuf:"bytes,25,opt,name=templateMessage" json:"templateMessage,omitempty"`
	StickerMessage                    *Message_StickerMessage                  `protobuf:"bytes,26,opt,name=stickerMessage" json:"stickerMessage,omitempty"`
	GroupInviteMessage                *Message_GroupInviteMessage              `protobuf:"bytes,28,opt,name=groupInviteMessage" json:"groupInviteMessage,omitempty"`
	TemplateButtonReplyMessage        *Message_TemplateButtonReplyMessage      `protobuf:"bytes,29,opt,name=templateButtonReplyMessage" json:"templateButtonReplyMessage,omitempty"`
	ProductMessage                    *Message_ProductMessage                  `protobuf:"bytes,30,opt,name=productMessage" json:"productMessage,omitempty"`
	DeviceSentMessage                 *Message_DeviceSentMessage               `protobuf:"bytes,31,opt,name=deviceSentMessage" json:"deviceSentMessage,omitempty"`
	MessageContextInfo                *MessageContextInfo                      `protobuf:"bytes,35,opt,name=messageContextInfo" json:"messageContextInfo,omitempty"`
	ListMessage                       *Message_ListMessage                     `protobuf:"bytes,36,opt,name=listMessage" json:"listMessage,omitempty"`
	ViewOnceMessage                   *Message_FutureProofMessage              `protobuf:"bytes,37,opt,name=viewOnceMessage" json:"viewOnceMessage,omitempty"`
	OrderMessage                      *Message_OrderMessage                    `protobuf:"bytes,38,opt,name=orderMessage" json:"orderMessage,omitempty"`
	ListResponseMessage               *Message_ListResponseMessage             `protobuf:"bytes,39,opt,name=listResponseMessage" json:"listResponseMessage,omitempty"`
	EphemeralMessage                  *Message_FutureProofMessage              `protobuf:"bytes,40,opt,name=ephemeralMessage" json:"ephemeralMessage,omitempty"`
	ButtonsMessage                    *Message_ButtonsMessage                  `protobuf:"bytes,42,opt,name=buttonsMessage" json:"buttonsMessage,omitempty"`
	ButtonsResponseMessage            *Message_ButtonsResponseMessage          `protobuf:"bytes,43,opt,name=buttonsResponseMessage" json:"buttonsResponseMessage,omitempty"`
	InteractiveMessage                *Message_InteractiveMessage              `protobuf:"bytes,45,opt,name=interactiveMessage" json:"interactiveMessage,omitempty"`
	ReactionMessage                   *Message_ReactionMessage                 `protobuf:"bytes,46,opt,name=reactionMessage" json:"reactionMessage,omitempty"`
	InteractiveResponseMessage        *Message_InteractiveResponseMessage      `protobuf:"bytes,48,opt,name=interactiveResponseMessage" json:"interactiveResponseMessage,omitempty"`
	PollCreationMessage               *Message_PollCreationMessage             `protobuf:"bytes,49,opt,name=pollCreationMessage" json:"pollCreationMessage,omitempty"`
	PollUpdateMessage                 *Message_PollUpdateMessage               `protobuf:"bytes,50,opt,name=pollUpdateMessage" json:"pollUpdateMessage,omitempty"`
	KeepInChatMessage                 *Message_KeepInChatMessage               `protobuf:"bytes,51,opt,name=keepInChatMessage" json:"keepInChatMessage,omitempty"`
	DocumentWithCaptionMessage        *Message_FutureProofMessage              `protobuf:"bytes,53,opt,name=documentWithCaptionMessage" json:"documentWithCaptionMessage,omitempty"`
	RequestPhoneNumberMessage         *Message_RequestPhoneNumberMessage       `protobuf:"bytes,54,opt,name=requestPhoneNumberMessage" json:"requestPhoneNumberMessage,omitempty"`
	ViewOnceMessageV2                 *Message_FutureProofMessage              `protobuf:"bytes,55,opt,name=viewOnceMessageV2" json:"viewOnceMessageV2,omitempty"`
	EncReactionMessage                *Message_EncReactionMessage              `protobuf:"bytes,56,opt,name=encReactionMessage" json:"encReactionMessage,omitempty"`
	EditedMessage                     *Message_FutureProofMessage              `protobuf:"bytes,58,opt,name=editedMessage" json:"editedMessage,omitempty"`
	ViewOnceMessageV2Extension        *Message_FutureProofMessage              `protobuf:"bytes,59,opt,name=viewOnceMessageV2Extension" json:"viewOnceMessageV2Extension,omitempty"`
	PollCreationMessageV2             *Message_PollCreationMessage             `protobuf:"bytes,60,opt,name=pollCreationMessageV2" json:"pollCreationMessageV2,omitempty"`
	GroupMentionedMessage             *Message_FutureProofMessage              `protobuf:"bytes,62,opt,name=groupMentionedMessage" json:"groupMentionedMessage,omitempty"`
	PinInChatMessage                  *Message_PinInChatMessage                `protobuf:"bytes,63,opt,name=pinInChatMessage" json:"pinInChatMessage,omitempty"`
	PollCreationMessageV3             *Message_PollCreationMessage             `protobuf:"bytes,64,opt,name=pollCreationMessageV3" json:"pollCreationMessageV3,omitempty"`
	PtvMessage                        *Message_VideoMessage                    `protobuf:"bytes,66,opt,name=ptvMessage" json:"ptvMessage,omitempty"`
	BotInvokeMessage                  *Message_FutureProofMessage              `protobuf:"bytes,67,opt,name=botInvokeMessage" json:"botInvokeMessage,omitempty"`
	MessageHistoryBundle              *Message_MessageHistoryBundle            `protobuf:"bytes,70,opt,name=messageHistoryBundle" json:"messageHistoryBundle,omitempty"`
	EncCommentMessage                 *Message_EncCommentMessage               `protobuf:"bytes,71,opt,name=encCommentMessage" json:"encCommentMessage,omitempty"`
	LottieStickerMessage              *Message_FutureProofMessage              `protobuf:"bytes,74,opt,name=lottieStickerMessage" json:"lottieStickerMessage,omitempty"`
	EventMessage                      *Message_EventMessage                    `protobuf:"bytes,75,opt,name=eventMessage" json:"eventMessage,omitempty"`
	EncEventResponseMessage           *Message_EncEventResponseMessage         `protobuf:"bytes,76,opt,name=encEventResponseMessage" json:"encEventResponseMessage,omitempty"`
	NewsletterAdminInviteMessage      *Message_NewsletterAdminInviteMessage    `protobuf:"bytes,78,opt,name=newsletterAdminInviteMessage" json:"newsletterAdminInviteMessage,omitempty"`
	SecretEncryptedMessage            *Message_SecretEncryptedMessage          `protobuf:"bytes,82,opt,name=secretEncryptedMessage" json:"secretEncryptedMessage,omitempty"`
	AlbumMessage                      *Message_AlbumMessage                    `protobuf:"bytes,83,opt,name=albumMessage" json:"albumMessage,omitempty"`
	StickerPackMessage                *Message_StickerPackMessage              `protobuf:"bytes,86,opt,name=stickerPackMessage" json:"stickerPackMessage,omitempty"`
	PollResultSnapshotMessage         *Message_PollResultSnapshotMessage       `protobuf:"bytes,88,opt,name=pollResultSnapshotMessage" json:"pollResultSnapshotMessage,omitempty"`
	PollCreationOptionImageMessage    *Message_FutureProofMessage              `protobuf:"bytes,90,opt,name=pollCreationOptionImageMessage" json:"pollCreationOptionImageMessage,omitempty"`
	AssociatedChildMessage            *Message_FutureProofMessage              `protobuf:"bytes,91,opt,name=associatedChildMessage" json:"associatedChildMessage,omitempty"`
	RichResponseMessage               *AIRichResponseMessage                   `protobuf:"bytes,97,opt,name=richResponseMessage" json:"richResponseMessage,omitempty"`
	QuestionMessage                   *Message_FutureProofMessage              `protobuf:"bytes,101,opt,name=questionMessage" json:"questionMessage,omitempty"`
	MessageHistoryNotice              *Message_MessageHistoryNotice            `protobuf:"bytes,102,opt,name=messageHistoryNotice" json:"messageHistoryNotice,omitempty"`
	BotForwardedMessage               *Message_FutureProofMessage              `protobuf:"bytes,104,opt,name=botForwardedMessage" json:"botForwardedMessage,omitempty"`
	QuestionReplyMessage              *Message_FutureProofMessage              `protobuf:"bytes,106,opt,name=questionReplyMessage" json:"questionReplyMessage,omitempty"`
	PollCreationMessageV5             *Message_PollCreationMessage             `protobuf:"bytes,111,opt,name=pollCreationMessageV5" json:"pollCreationMessageV5,omitempty"`
	NewsletterFollowerInviteMessageV2 *Message_NewsletterFollowerInviteMessage `protobuf:"bytes,113,opt,name=newsletterFollowerInviteMessageV2" json:"newsletterFollowerInviteMessageV2,omitempty"`
	PollResultSnapshotMessageV3       *Message_PollResultSnapshotMessage       `protobuf:"bytes,115,opt,name=pollResultSnapshotMessageV3" json:"pollResultSnapshotMessageV3,omitempty"`
	SpoilerMessage                    *Message_FutureProofMessage              `protobuf:"bytes,118,opt,name=spoilerMessage" json:"spoilerMessage,omitempty"`
	PollCreationMessageV6             *Message_PollCreationMessage             `protobuf:"bytes,119,opt,name=pollCreationMessageV6" json:"pollCreationMessageV6,omitempty"`
	EventInviteMessage                *Message_EventInviteMessage              `protobuf:"bytes,122,opt,name=eventInviteMessage" json:"eventInviteMessage,omitempty"`
	SplitPaymentMessage               *Message_SplitPaymentMessage             `protobuf:"bytes,125,opt,name=splitPaymentMessage" json:"splitPaymentMessage,omitempty"`
	MusicMessage                      *Message_MusicMessage                    `protobuf:"bytes,129,opt,name=musicMessage" json:"musicMessage,omitempty"`
	unknownFields                     protoimpl.UnknownFields
	sizeCache                         protoimpl.SizeCache
}

func (x *Message) Reset() {
	*x = Message{}
	mi := &file_chatwire_wire_proto_msgTypes[14]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message) ProtoMessage() {}

func (x *Message) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[14]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14}
}

func (x *Message) GetConversation() string {
	if x != nil && x.Conversation != nil {
		return *x.Conversation
	}
	return ""
}

func (x *Message) GetSenderKeyDistributionMessage() *Message_SenderKeyDistributionMessage {
	if x != nil {
		return x.SenderKeyDistributionMessage
	}
	return nil
}

func (x *Message) GetImageMessage() *Message_ImageMessage {
	if x != nil {
		return x.ImageMessage
	}
	return nil
}

func (x *Message) GetContactMessage() *Message_ContactMessage {
	if x != nil {
		return x.ContactMessage
	}
	return nil
}

func (x *Message) GetLocationMessage() *Message_LocationMessage {
	if x != nil {
		return x.LocationMessage
	}
	return nil
}

func (x *Message) GetExtendedTextMessage() *Message_ExtendedTextMessage {
	if x != nil {
		return x.ExtendedTextMessage
	}
	return nil
}

func (x *Message) GetDocumentMessage() *Message_DocumentMessage {
	if x != nil {
		return x.DocumentMessage
	}
	return nil
}

func (x *Message) GetAudioMessage() *Message_AudioMessage {
	if x != nil {
		return x.AudioMessage
	}
	return nil
}

func (x *Message) GetVideoMessage() *Message_VideoMessage {
	if x != nil {
		return x.VideoMessage
	}
	return nil
}

func (x *Message) GetCall() *Message_Call {
	if x != nil {
		return x.Call
	}
	return nil
}

func (x *Message) GetProtocolMessage() *Message_ProtocolMessage {
	if x != nil {
		return x.ProtocolMessage
	}
	return nil
}

func (x *Message) GetContactsArrayMessage() *Message_ContactsArrayMessage {
	if x != nil {
		return x.ContactsArrayMessage
	}
	return nil
}

func (x *Message) GetLiveLocationMessage() *Message_LiveLocationMessage {
	if x != nil {
		return x.LiveLocationMessage
	}
	return nil
}

func (x *Message) GetTemplateMessage() *Message_TemplateMessage {
	if x != nil {
		return x.TemplateMessage
	}
	return nil
}

func (x *Message) GetStickerMessage() *Message_StickerMessage {
	if x != nil {
		return x.StickerMessage
	}
	return nil
}

func (x *Message) GetGroupInviteMessage() *Message_GroupInviteMessage {
	if x != nil {
		return x.GroupInviteMessage
	}
	return nil
}

func (x *Message) GetTemplateButtonReplyMessage() *Message_TemplateButtonReplyMessage {
	if x != nil {
		return x.TemplateButtonReplyMessage
	}
	return nil
}

func (x *Message) GetProductMessage() *Message_ProductMessage {
	if x != nil {
		return x.ProductMessage
	}
	return nil
}

func (x *Message) GetDeviceSentMessage() *Message_DeviceSentMessage {
	if x != nil {
		return x.DeviceSentMessage
	}
	return nil
}

func (x *Message) GetMessageContextInfo() *MessageContextInfo {
	if x != nil {
		return x.MessageContextInfo
	}
	return nil
}

func (x *Message) GetListMessage() *Message_ListMessage {
	if x != nil {
		return x.ListMessage
	}
	return nil
}

func (x *Message) GetViewOnceMessage() *Message_FutureProofMessage {
	if x != nil {
		return x.ViewOnceMessage
	}
	return nil
}

func (x *Message) GetOrderMessage() *Message_OrderMessage {
	if x != nil {
		return x.OrderMessage
	}
	return nil
}

func (x *Message) GetListResponseMessage() *Message_ListResponseMessage {
	if x != nil {
		return x.ListResponseMessage
	}
	return nil
}

func (x *Message) GetEphemeralMessage() *Message_FutureProofMessage {
	if x != nil {
		return x.EphemeralMessage
	}
	return nil
}

func (x *Message) GetButtonsMessage() *Message_ButtonsMessage {
	if x != nil {
		return x.ButtonsMessage
	}
	return nil
}

func (x *Message) GetButtonsResponseMessage() *Message_ButtonsResponseMessage {
	if x != nil {
		return x.ButtonsResponseMessage
	}
	return nil
}

func (x *Message) GetInteractiveMessage() *Message_InteractiveMessage {
	if x != nil {
		return x.InteractiveMessage
	}
	return nil
}

func (x *Message) GetReactionMessage() *Message_ReactionMessage {
	if x != nil {
		return x.ReactionMessage
	}
	return nil
}

func (x *Message) GetInteractiveResponseMessage() *Message_InteractiveResponseMessage {
	if x != nil {
		return x.InteractiveResponseMessage
	}
	return nil
}

func (x *Message) GetPollCreationMessage() *Message_PollCreationMessage {
	if x != nil {
		return x.PollCreationMessage
	}
	return nil
}

func (x *Message) GetPollUpdateMessage() *Message_PollUpdateMessage {
	if x != nil {
		return x.PollUpdateMessage
	}
	return nil
}

func (x *Message) GetKeepInChatMessage() *Message_KeepInChatMessage {
	if x != nil {
		return x.KeepInChatMessage
	}
	return nil
}

func (x *Message) GetDocumentWithCaptionMessage() *Message_FutureProofMessage {
	if x != nil {
		return x.DocumentWithCaptionMessage
	}
	return nil
}

func (x *Message) GetRequestPhoneNumberMessage() *Message_RequestPhoneNumberMessage {
	if x != nil {
		return x.RequestPhoneNumberMessage
	}
	return nil
}

func (x *Message) GetViewOnceMessageV2() *Message_FutureProofMessage {
	if x != nil {
		return x.ViewOnceMessageV2
	}
	return nil
}

func (x *Message) GetEncReactionMessage() *Message_EncReactionMessage {
	if x != nil {
		return x.EncReactionMessage
	}
	return nil
}

func (x *Message) GetEditedMessage() *Message_FutureProofMessage {
	if x != nil {
		return x.EditedMessage
	}
	return nil
}

func (x *Message) GetViewOnceMessageV2Extension() *Message_FutureProofMessage {
	if x != nil {
		return x.ViewOnceMessageV2Extension
	}
	return nil
}

func (x *Message) GetPollCreationMessageV2() *Message_PollCreationMessage {
	if x != nil {
		return x.PollCreationMessageV2
	}
	return nil
}

func (x *Message) GetGroupMentionedMessage() *Message_FutureProofMessage {
	if x != nil {
		return x.GroupMentionedMessage
	}
	return nil
}

func (x *Message) GetPinInChatMessage() *Message_PinInChatMessage {
	if x != nil {
		return x.PinInChatMessage
	}
	return nil
}

func (x *Message) GetPollCreationMessageV3() *Message_PollCreationMessage {
	if x != nil {
		return x.PollCreationMessageV3
	}
	return nil
}

func (x *Message) GetPtvMessage() *Message_VideoMessage {
	if x != nil {
		return x.PtvMessage
	}
	return nil
}

func (x *Message) GetBotInvokeMessage() *Message_FutureProofMessage {
	if x != nil {
		return x.BotInvokeMessage
	}
	return nil
}

func (x *Message) GetMessageHistoryBundle() *Message_MessageHistoryBundle {
	if x != nil {
		return x.MessageHistoryBundle
	}
	return nil
}

func (x *Message) GetEncCommentMessage() *Message_EncCommentMessage {
	if x != nil {
		return x.EncCommentMessage
	}
	return nil
}

func (x *Message) GetLottieStickerMessage() *Message_FutureProofMessage {
	if x != nil {
		return x.LottieStickerMessage
	}
	return nil
}

func (x *Message) GetEventMessage() *Message_EventMessage {
	if x != nil {
		return x.EventMessage
	}
	return nil
}

func (x *Message) GetEncEventResponseMessage() *Message_EncEventResponseMessage {
	if x != nil {
		return x.EncEventResponseMessage
	}
	return nil
}

func (x *Message) GetNewsletterAdminInviteMessage() *Message_NewsletterAdminInviteMessage {
	if x != nil {
		return x.NewsletterAdminInviteMessage
	}
	return nil
}

func (x *Message) GetSecretEncryptedMessage() *Message_SecretEncryptedMessage {
	if x != nil {
		return x.SecretEncryptedMessage
	}
	return nil
}

func (x *Message) GetAlbumMessage() *Message_AlbumMessage {
	if x != nil {
		return x.AlbumMessage
	}
	return nil
}

func (x *Message) GetStickerPackMessage() *Message_StickerPackMessage {
	if x != nil {
		return x.StickerPackMessage
	}
	return nil
}

func (x *Message) GetPollResultSnapshotMessage() *Message_PollResultSnapshotMessage {
	if x != nil {
		return x.PollResultSnapshotMessage
	}
	return nil
}

func (x *Message) GetPollCreationOptionImageMessage() *Message_FutureProofMessage {
	if x != nil {
		return x.PollCreationOptionImageMessage
	}
	return nil
}

func (x *Message) GetAssociatedChildMessage() *Message_FutureProofMessage {
	if x != nil {
		return x.AssociatedChildMessage
	}
	return nil
}

func (x *Message) GetRichResponseMessage() *AIRichResponseMessage {
	if x != nil {
		return x.RichResponseMessage
	}
	return nil
}

func (x *Message) GetQuestionMessage() *Message_FutureProofMessage {
	if x != nil {
		return x.QuestionMessage
	}
	return nil
}

func (x *Message) GetMessageHistoryNotice() *Message_MessageHistoryNotice {
	if x != nil {
		return x.MessageHistoryNotice
	}
	return nil
}

func (x *Message) GetBotForwardedMessage() *Message_FutureProofMessage {
	if x != nil {
		return x.BotForwardedMessage
	}
	return nil
}

func (x *Message) GetQuestionReplyMessage() *Message_FutureProofMessage {
	if x != nil {
		return x.QuestionReplyMessage
	}
	return nil
}

func (x *Message) GetPollCreationMessageV5() *Message_PollCreationMessage {
	if x != nil {
		return x.PollCreationMessageV5
	}
	return nil
}

func (x *Message) GetNewsletterFollowerInviteMessageV2() *Message_NewsletterFollowerInviteMessage {
	if x != nil {
		return x.NewsletterFollowerInviteMessageV2
	}
	return nil
}

func (x *Message) GetPollResultSnapshotMessageV3() *Message_PollResultSnapshotMessage {
	if x != nil {
		return x.PollResultSnapshotMessageV3
	}
	return nil
}

func (x *Message) GetSpoilerMessage() *Message_FutureProofMessage {
	if x != nil {
		return x.SpoilerMessage
	}
	return nil
}

func (x *Message) GetPollCreationMessageV6() *Message_PollCreationMessage {
	if x != nil {
		return x.PollCreationMessageV6
	}
	return nil
}

func (x *Message) GetEventInviteMessage() *Message_EventInviteMessage {
	if x != nil {
		return x.EventInviteMessage
	}
	return nil
}

func (x *Message) GetSplitPaymentMessage() *Message_SplitPaymentMessage {
	if x != nil {
		return x.SplitPaymentMessage
	}
	return nil
}

func (x *Message) GetMusicMessage() *Message_MusicMessage {
	if x != nil {
		return x.MusicMessage
	}
	return nil
}

type MessageAddOn struct {
	state             protoimpl.MessageState         `protogen:"open.v1"`
	MessageAddOnType  *MessageAddOn_MessageAddOnType `protobuf:"varint,1,opt,name=messageAddOnType,enum=chatwire.wire.MessageAddOn_MessageAddOnType" json:"messageAddOnType,omitempty"`
	SenderTimestampMs *int64                         `protobuf:"varint,3,opt,name=senderTimestampMs" json:"senderTimestampMs,omitempty"`
	MessageAddOnKey   *MessageKey                    `protobuf:"bytes,7,opt,name=messageAddOnKey" json:"messageAddOnKey,omitempty"`
	LegacyMessage     *LegacyMessage                 `protobuf:"bytes,8,opt,name=legacyMessage" json:"legacyMessage,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *MessageAddOn) Reset() {
	*x = MessageAddOn{}
	mi := &file_chatwire_wire_proto_msgTypes[15]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MessageAddOn) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MessageAddOn) ProtoMessage() {}

func (x *MessageAddOn) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[15]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*MessageAddOn) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{15}
}

func (x *MessageAddOn) GetMessageAddOnType() MessageAddOn_MessageAddOnType {
	if x != nil && x.MessageAddOnType != nil {
		return *x.MessageAddOnType
	}
	return MessageAddOn_UNDEFINED
}

func (x *MessageAddOn) GetSenderTimestampMs() int64 {
	if x != nil && x.SenderTimestampMs != nil {
		return *x.SenderTimestampMs
	}
	return 0
}

func (x *MessageAddOn) GetMessageAddOnKey() *MessageKey {
	if x != nil {
		return x.MessageAddOnKey
	}
	return nil
}

func (x *MessageAddOn) GetLegacyMessage() *LegacyMessage {
	if x != nil {
		return x.LegacyMessage
	}
	return nil
}

type MessageContextInfo struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	MessageSecret []byte                 `protobuf:"bytes,3,opt,name=messageSecret" json:"messageSecret,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *MessageContextInfo) Reset() {
	*x = MessageContextInfo{}
	mi := &file_chatwire_wire_proto_msgTypes[16]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MessageContextInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MessageContextInfo) ProtoMessage() {}

func (x *MessageContextInfo) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[16]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*MessageContextInfo) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{16}
}

func (x *MessageContextInfo) GetMessageSecret() []byte {
	if x != nil {
		return x.MessageSecret
	}
	return nil
}

type MessageInfo struct {
	state            protoimpl.MessageState `protogen:"open.v1"`
	Key              *MessageKey            `protobuf:"bytes,1,req,name=key" json:"key,omitempty"`
	Message          *Message               `protobuf:"bytes,2,opt,name=message" json:"message,omitempty"`
	MessageTimestamp *uint64                `protobuf:"varint,3,opt,name=messageTimestamp" json:"messageTimestamp,omitempty"`
	Participant      *string                `protobuf:"bytes,5,opt,name=participant" json:"participant,omitempty"`
	PushName         *string                `protobuf:"bytes,19,opt,name=pushName" json:"pushName,omitempty"`
	Reactions        []*Reaction            `protobuf:"bytes,41,rep,name=reactions" json:"reactions,omitempty"`
	PollUpdates      []*PollUpdate          `protobuf:"bytes,45,rep,name=pollUpdates" json:"pollUpdates,omitempty"`
	MessageSecret    []byte                 `protobuf:"bytes,49,opt,name=messageSecret" json:"messageSecret,omitempty"`
	MessageAddOns    []*MessageAddOn        `protobuf:"bytes,68,rep,name=messageAddOns" json:"messageAddOns,omitempty"`
	unknownFields    protoimpl.UnknownFields
	sizeCache        protoimpl.SizeCache
}

func (x *MessageInfo) Reset() {
	*x = MessageInfo{}
	mi := &file_chatwire_wire_proto_msgTypes[17]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MessageInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MessageInfo) ProtoMessage() {}

func (x *MessageInfo) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[17]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*MessageInfo) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{17}
}

func (x *MessageInfo) GetKey() *MessageKey {
	if x != nil {
		return x.Key
	}
	return nil
}

func (x *MessageInfo) GetMessage() *Message {
	if x != nil {
		return x.Message
	}
	return nil
}

func (x *MessageInfo) GetMessageTimestamp() uint64 {
	if x != nil && x.MessageTimestamp != nil {
		return *x.MessageTimestamp
	}
	return 0
}

func (x *MessageInfo) GetParticipant() string {
	if x != nil && x.Participant != nil {
		return *x.Participant
	}
	return ""
}

func (x *MessageInfo) GetPushName() string {
	if x != nil && x.PushName != nil {
		return *x.PushName
	}
	return ""
}

func (x *MessageInfo) GetReactions() []*Reaction {
	if x != nil {
		return x.Reactions
	}
	return nil
}

func (x *MessageInfo) GetPollUpdates() []*PollUpdate {
	if x != nil {
		return x.PollUpdates
	}
	return nil
}

func (x *MessageInfo) GetMessageSecret() []byte {
	if x != nil {
		return x.MessageSecret
	}
	return nil
}

func (x *MessageInfo) GetMessageAddOns() []*MessageAddOn {
	if x != nil {
		return x.MessageAddOns
	}
	return nil
}

type MessageKey struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RemoteJid     *string                `protobuf:"bytes,1,opt,name=remoteJid" json:"remoteJid,omitempty"`
	FromMe        *bool                  `protobuf:"varint,2,opt,name=fromMe" json:"fromMe,omitempty"`
	Id            *string                `protobuf:"bytes,3,opt,name=id" json:"id,omitempty"`
	Participant   *string                `protobuf:"bytes,4,opt,name=participant" json:"participant,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *MessageKey) Reset() {
	*x = MessageKey{}
	mi := &file_chatwire_wire_proto_msgTypes[18]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MessageKey) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MessageKey) ProtoMessage() {}

func (x *MessageKey) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[18]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*MessageKey) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{18}
}

func (x *MessageKey) GetRemoteJid() string {
	if x != nil && x.RemoteJid != nil {
		return *x.RemoteJid
	}
	return ""
}

func (x *MessageKey) GetFromMe() bool {
	if x != nil && x.FromMe != nil {
		return *x.FromMe
	}
	return false
}

func (x *MessageKey) GetId() string {
	if x != nil && x.Id != nil {
		return *x.Id
	}
	return ""
}

func (x *MessageKey) GetParticipant() string {
	if x != nil && x.Participant != nil {
		return *x.Participant
	}
	return ""
}

type PhoneNumberToLIDMapping struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	PnJid         *string                `protobuf:"bytes,1,opt,name=pnJid" json:"pnJid,omitempty"`
	LidJid        *string                `protobuf:"bytes,2,opt,name=lidJid" json:"lidJid,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *PhoneNumberToLIDMapping) Reset() {
	*x = PhoneNumberToLIDMapping{}
	mi := &file_chatwire_wire_proto_msgTypes[19]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PhoneNumberToLIDMapping) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PhoneNumberToLIDMapping) ProtoMessage() {}

func (x *PhoneNumberToLIDMapping) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[19]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*PhoneNumberToLIDMapping) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{19}
}

func (x *PhoneNumberToLIDMapping) GetPnJid() string {
	if x != nil && x.PnJid != nil {
		return *x.PnJid
	}
	return ""
}

func (x *PhoneNumberToLIDMapping) GetLidJid() string {
	if x != nil && x.LidJid != nil {
		return *x.LidJid
	}
	return ""
}

type PollUpdate struct {
	state                protoimpl.MessageState   `protogen:"open.v1"`
	PollUpdateMessageKey *MessageKey              `protobuf:"bytes,1,opt,name=pollUpdateMessageKey" json:"pollUpdateMessageKey,omitempty"`
	Vote                 *Message_PollVoteMessage `protobuf:"bytes,2,opt,name=vote" json:"vote,omitempty"`
	SenderTimestampMs    *int64                   `protobuf:"varint,3,opt,name=senderTimestampMs" json:"senderTimestampMs,omitempty"`
	unknownFields        protoimpl.UnknownFields
	sizeCache            protoimpl.SizeCache
}

func (x *PollUpdate) Reset() {
	*x = PollUpdate{}
	mi := &file_chatwire_wire_proto_msgTypes[20]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PollUpdate) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PollUpdate) ProtoMessage() {}

func (x *PollUpdate) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[20]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*PollUpdate) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{20}
}

func (x *PollUpdate) GetPollUpdateMessageKey() *MessageKey {
	if x != nil {
		return x.PollUpdateMessageKey
	}
	return nil
}

func (x *PollUpdate) GetVote() *Message_PollVoteMessage {
	if x != nil {
		return x.Vote
	}
	return nil
}

func (x *PollUpdate) GetSenderTimestampMs() int64 {
	if x != nil && x.SenderTimestampMs != nil {
		return *x.SenderTimestampMs
	}
	return 0
}

type PreKeySignalMessage struct {
	state          protoimpl.MessageState `protogen:"open.v1"`
	PreKeyId       *uint32                `protobuf:"varint,1,opt,name=preKeyId" json:"preKeyId,omitempty"`
	BaseKey        []byte                 `protobuf:"bytes,2,opt,name=baseKey" json:"baseKey,omitempty"`
	IdentityKey    []byte                 `protobuf:"bytes,3,opt,name=identityKey" json:"identityKey,omitempty"`
	Message        []byte                 `protobuf:"bytes,4,opt,name=message" json:"message,omitempty"`
	RegistrationId *uint32                `protobuf:"varint,5,opt,name=registrationId" json:"registrationId,omitempty"`
	SignedPreKeyId *uint32                `protobuf:"varint,6,opt,name=signedPreKeyId" json:"signedPreKeyId,omitempty"`
	unknownFields  protoimpl.UnknownFields
	sizeCache      protoimpl.SizeCache
}

func (x *PreKeySignalMessage) Reset() {
	*x = PreKeySignalMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[21]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PreKeySignalMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PreKeySignalMessage) ProtoMessage() {}

func (x *PreKeySignalMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[21]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*PreKeySignalMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{21}
}

func (x *PreKeySignalMessage) GetPreKeyId() uint32 {
	if x != nil && x.PreKeyId != nil {
		return *x.PreKeyId
	}
	return 0
}

func (x *PreKeySignalMessage) GetBaseKey() []byte {
	if x != nil {
		return x.BaseKey
	}
	return nil
}

func (x *PreKeySignalMessage) GetIdentityKey() []byte {
	if x != nil {
		return x.IdentityKey
	}
	return nil
}

func (x *PreKeySignalMessage) GetMessage() []byte {
	if x != nil {
		return x.Message
	}
	return nil
}

func (x *PreKeySignalMessage) GetRegistrationId() uint32 {
	if x != nil && x.RegistrationId != nil {
		return *x.RegistrationId
	}
	return 0
}

func (x *PreKeySignalMessage) GetSignedPreKeyId() uint32 {
	if x != nil && x.SignedPreKeyId != nil {
		return *x.SignedPreKeyId
	}
	return 0
}

type Pushname struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            *string                `protobuf:"bytes,1,opt,name=id" json:"id,omitempty"`
	Pushname      *string                `protobuf:"bytes,2,opt,name=pushname" json:"pushname,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Pushname) Reset() {
	*x = Pushname{}
	mi := &file_chatwire_wire_proto_msgTypes[22]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Pushname) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Pushname) ProtoMessage() {}

func (x *Pushname) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[22]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Pushname) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{22}
}

func (x *Pushname) GetId() string {
	if x != nil && x.Id != nil {
		return *x.Id
	}
	return ""
}

func (x *Pushname) GetPushname() string {
	if x != nil && x.Pushname != nil {
		return *x.Pushname
	}
	return ""
}

type Reaction struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	Key               *MessageKey            `protobuf:"bytes,1,opt,name=key" json:"key,omitempty"`
	Text              *string                `protobuf:"bytes,2,opt,name=text" json:"text,omitempty"`
	SenderTimestampMs *int64                 `protobuf:"varint,4,opt,name=senderTimestampMs" json:"senderTimestampMs,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *Reaction) Reset() {
	*x = Reaction{}
	mi := &file_chatwire_wire_proto_msgTypes[23]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Reaction) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Reaction) ProtoMessage() {}

func (x *Reaction) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[23]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Reaction) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{23}
}

func (x *Reaction) GetKey() *MessageKey {
	if x != nil {
		return x.Key
	}
	return nil
}

func (x *Reaction) GetText() string {
	if x != nil && x.Text != nil {
		return *x.Text
	}
	return ""
}

func (x *Reaction) GetSenderTimestampMs() int64 {
	if x != nil && x.SenderTimestampMs != nil {
		return *x.SenderTimestampMs
	}
	return 0
}

type SenderKeyDistributionMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            *uint32                `protobuf:"varint,1,opt,name=id" json:"id,omitempty"`
	Iteration     *uint32                `protobuf:"varint,2,opt,name=iteration" json:"iteration,omitempty"`
	ChainKey      []byte                 `protobuf:"bytes,3,opt,name=chainKey" json:"chainKey,omitempty"`
	SigningKey    []byte                 `protobuf:"bytes,4,opt,name=signingKey" json:"signingKey,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SenderKeyDistributionMessage) Reset() {
	*x = SenderKeyDistributionMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[24]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SenderKeyDistributionMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SenderKeyDistributionMessage) ProtoMessage() {}

func (x *SenderKeyDistributionMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[24]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SenderKeyDistributionMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{24}
}

func (x *SenderKeyDistributionMessage) GetId() uint32 {
	if x != nil && x.Id != nil {
		return *x.Id
	}
	return 0
}

func (x *SenderKeyDistributionMessage) GetIteration() uint32 {
	if x != nil && x.Iteration != nil {
		return *x.Iteration
	}
	return 0
}

func (x *SenderKeyDistributionMessage) GetChainKey() []byte {
	if x != nil {
		return x.ChainKey
	}
	return nil
}

func (x *SenderKeyDistributionMessage) GetSigningKey() []byte {
	if x != nil {
		return x.SigningKey
	}
	return nil
}

type SenderKeyMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            *uint32                `protobuf:"varint,1,opt,name=id" json:"id,omitempty"`
	Iteration     *uint32                `protobuf:"varint,2,opt,name=iteration" json:"iteration,omitempty"`
	Ciphertext    []byte                 `protobuf:"bytes,3,opt,name=ciphertext" json:"ciphertext,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SenderKeyMessage) Reset() {
	*x = SenderKeyMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[25]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SenderKeyMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SenderKeyMessage) ProtoMessage() {}

func (x *SenderKeyMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[25]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SenderKeyMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{25}
}

func (x *SenderKeyMessage) GetId() uint32 {
	if x != nil && x.Id != nil {
		return *x.Id
	}
	return 0
}

func (x *SenderKeyMessage) GetIteration() uint32 {
	if x != nil && x.Iteration != nil {
		return *x.Iteration
	}
	return 0
}

func (x *SenderKeyMessage) GetCiphertext() []byte {
	if x != nil {
		return x.Ciphertext
	}
	return nil
}

type ServerErrorReceipt struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	StanzaId      *string                `protobuf:"bytes,1,opt,name=stanzaId" json:"stanzaId,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ServerErrorReceipt) Reset() {
	*x = ServerErrorReceipt{}
	mi := &file_chatwire_wire_proto_msgTypes[26]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ServerErrorReceipt) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ServerErrorReceipt) ProtoMessage() {}

func (x *ServerErrorReceipt) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[26]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ServerErrorReceipt) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{26}
}

func (x *ServerErrorReceipt) GetStanzaId() string {
	if x != nil && x.StanzaId != nil {
		return *x.StanzaId
	}
	return ""
}

type SignalMessage struct {
	state           protoimpl.MessageState `protogen:"open.v1"`
	RatchetKey      []byte                 `protobuf:"bytes,1,opt,name=ratchetKey" json:"ratchetKey,omitempty"`
	Counter         *uint32                `protobuf:"varint,2,opt,name=counter" json:"counter,omitempty"`
	PreviousCounter *uint32                `protobuf:"varint,3,opt,name=previousCounter" json:"previousCounter,omitempty"`
	Ciphertext      []byte                 `protobuf:"bytes,4,opt,name=ciphertext" json:"ciphertext,omitempty"`
	unknownFields   protoimpl.UnknownFields
	sizeCache       protoimpl.SizeCache
}

func (x *SignalMessage) Reset() {
	*x = SignalMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[27]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SignalMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignalMessage) ProtoMessage() {}

func (x *SignalMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[27]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SignalMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{27}
}

func (x *SignalMessage) GetRatchetKey() []byte {
	if x != nil {
		return x.RatchetKey
	}
	return nil
}

func (x *SignalMessage) GetCounter() uint32 {
	if x != nil && x.Counter != nil {
		return *x.Counter
	}
	return 0
}

func (x *SignalMessage) GetPreviousCounter() uint32 {
	if x != nil && x.PreviousCounter != nil {
		return *x.PreviousCounter
	}
	return 0
}

func (x *SignalMessage) GetCiphertext() []byte {
	if x != nil {
		return x.Ciphertext
	}
	return nil
}

type SyncActionData struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Index         []byte                 `protobuf:"bytes,1,opt,name=index" json:"index,omitempty"`
	Value         *SyncActionValue       `protobuf:"bytes,2,opt,name=value" json:"value,omitempty"`
	Padding       []byte                 `protobuf:"bytes,3,opt,name=padding" json:"padding,omitempty"`
	Version       *int32                 `protobuf:"varint,4,opt,name=version" json:"version,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncActionData) Reset() {
	*x = SyncActionData{}
	mi := &file_chatwire_wire_proto_msgTypes[28]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncActionData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncActionData) ProtoMessage() {}

func (x *SyncActionData) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[28]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SyncActionData) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{28}
}

func (x *SyncActionData) GetIndex() []byte {
	if x != nil {
		return x.Index
	}
	return nil
}

func (x *SyncActionData) GetValue() *SyncActionValue {
	if x != nil {
		return x.Value
	}
	return nil
}

func (x *SyncActionData) GetPadding() []byte {
	if x != nil {
		return x.Padding
	}
	return nil
}

func (x *SyncActionData) GetVersion() int32 {
	if x != nil && x.Version != nil {
		return *x.Version
	}
	return 0
}

type SyncActionValue struct {
	state             protoimpl.MessageState             `protogen:"open.v1"`
	Timestamp         *int64                             `protobuf:"varint,1,opt,name=timestamp" json:"timestamp,omitempty"`
	ContactAction     *SyncActionValue_ContactAction     `protobuf:"bytes,3,opt,name=contactAction" json:"contactAction,omitempty"`
	MuteAction        *SyncActionValue_MuteAction        `protobuf:"bytes,4,opt,name=muteAction" json:"muteAction,omitempty"`
	PinAction         *SyncActionValue_PinAction         `protobuf:"bytes,5,opt,name=pinAction" json:"pinAction,omitempty"`
	ArchiveChatAction *SyncActionValue_ArchiveChatAction `protobuf:"bytes,17,opt,name=archiveChatAction" json:"archiveChatAction,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *SyncActionValue) Reset() {
	*x = SyncActionValue{}
	mi := &file_chatwire_wire_proto_msgTypes[29]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncActionValue) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncActionValue) ProtoMessage() {}

func (x *SyncActionValue) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[29]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SyncActionValue) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{29}
}

func (x *SyncActionValue) GetTimestamp() int64 {
	if x != nil && x.Timestamp != nil {
		return *x.Timestamp
	}
	return 0
}

func (x *SyncActionValue) GetContactAction() *SyncActionValue_ContactAction {
	if x != nil {
		return x.ContactAction
	}
	return nil
}

func (x *SyncActionValue) GetMuteAction() *SyncActionValue_MuteAction {
	if x != nil {
		return x.MuteAction
	}
	return nil
}

func (x *SyncActionValue) GetPinAction() *SyncActionValue_PinAction {
	if x != nil {
		return x.PinAction
	}
	return nil
}

func (x *SyncActionValue) GetArchiveChatAction() *SyncActionValue_ArchiveChatAction {
	if x != nil {
		return x.ArchiveChatAction
	}
	return nil
}

type SyncdIndex struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Blob          []byte                 `protobuf:"bytes,1,opt,name=blob" json:"blob,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncdIndex) Reset() {
	*x = SyncdIndex{}
	mi := &file_chatwire_wire_proto_msgTypes[30]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdIndex) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdIndex) ProtoMessage() {}

func (x *SyncdIndex) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[30]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SyncdIndex) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{30}
}

func (x *SyncdIndex) GetBlob() []byte {
	if x != nil {
		return x.Blob
	}
	return nil
}

type SyncdMutation struct {
	state         protoimpl.MessageState        `protogen:"open.v1"`
	Operation     *SyncdMutation_SyncdOperation `protobuf:"varint,1,opt,name=operation,enum=chatwire.wire.SyncdMutation_SyncdOperation" json:"operation,omitempty"`
	Record        *SyncdRecord                  `protobuf:"bytes,2,opt,name=record" json:"record,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncdMutation) Reset() {
	*x = SyncdMutation{}
	mi := &file_chatwire_wire_proto_msgTypes[31]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdMutation) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdMutation) ProtoMessage() {}

func (x *SyncdMutation) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[31]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SyncdMutation) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{31}
}

func (x *SyncdMutation) GetOperation() SyncdMutation_SyncdOperation {
	if x != nil && x.Operation != nil {
		return *x.Operation
	}
	return SyncdMutation_SET
}

func (x *SyncdMutation) GetRecord() *SyncdRecord {
	if x != nil {
		return x.Record
	}
	return nil
}

type SyncdMutations struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Mutations     []*SyncdMutation       `protobuf:"bytes,1,rep,name=mutations" json:"mutations,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncdMutations) Reset() {
	*x = SyncdMutations{}
	mi := &file_chatwire_wire_proto_msgTypes[32]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdMutations) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdMutations) ProtoMessage() {}

func (x *SyncdMutations) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[32]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SyncdMutations) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{32}
}

func (x *SyncdMutations) GetMutations() []*SyncdMutation {
	if x != nil {
		return x.Mutations
	}
	return nil
}

type SyncdPatch struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	Version           *SyncdVersion          `protobuf:"bytes,1,opt,name=version" json:"version,omitempty"`
	Mutations         []*SyncdMutation       `protobuf:"bytes,2,rep,name=mutations" json:"mutations,omitempty"`
	ExternalMutations *ExternalBlobReference `protobuf:"bytes,3,opt,name=externalMutations" json:"externalMutations,omitempty"`
	SnapshotMac       []byte                 `protobuf:"bytes,4,opt,name=snapshotMac" json:"snapshotMac,omitempty"`
	PatchMac          []byte                 `protobuf:"bytes,5,opt,name=patchMac" json:"patchMac,omitempty"`
	KeyId             *KeyId                 `protobuf:"bytes,6,opt,name=keyId" json:"keyId,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *SyncdPatch) Reset() {
	*x = SyncdPatch{}
	mi := &file_chatwire_wire_proto_msgTypes[33]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdPatch) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdPatch) ProtoMessage() {}

func (x *SyncdPatch) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[33]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SyncdPatch) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{33}
}

func (x *SyncdPatch) GetVersion() *SyncdVersion {
	if x != nil {
		return x.Version
	}
	return nil
}

func (x *SyncdPatch) GetMutations() []*SyncdMutation {
	if x != nil {
		return x.Mutations
	}
	return nil
}

func (x *SyncdPatch) GetExternalMutations() *ExternalBlobReference {
	if x != nil {
		return x.ExternalMutations
	}
	return nil
}

func (x *SyncdPatch) GetSnapshotMac() []byte {
	if x != nil {
		return x.SnapshotMac
	}
	return nil
}

func (x *SyncdPatch) GetPatchMac() []byte {
	if x != nil {
		return x.PatchMac
	}
	return nil
}

func (x *SyncdPatch) GetKeyId() *KeyId {
	if x != nil {
		return x.KeyId
	}
	return nil
}

type SyncdRecord struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Index         *SyncdIndex            `protobuf:"bytes,1,opt,name=index" json:"index,omitempty"`
	Value         *SyncdValue            `protobuf:"bytes,2,opt,name=value" json:"value,omitempty"`
	KeyId         *KeyId                 `protobuf:"bytes,3,opt,name=keyId" json:"keyId,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncdRecord) Reset() {
	*x = SyncdRecord{}
	mi := &file_chatwire_wire_proto_msgTypes[34]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdRecord) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdRecord) ProtoMessage() {}

func (x *SyncdRecord) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[34]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SyncdRecord) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{34}
}

func (x *SyncdRecord) GetIndex() *SyncdIndex {
	if x != nil {
		return x.Index
	}
	return nil
}

func (x *SyncdRecord) GetValue() *SyncdValue {
	if x != nil {
		return x.Value
	}
	return nil
}

func (x *SyncdRecord) GetKeyId() *KeyId {
	if x != nil {
		return x.KeyId
	}
	return nil
}

type SyncdSnapshot struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Version       *SyncdVersion          `protobuf:"bytes,1,opt,name=version" json:"version,omitempty"`
	Records       []*SyncdRecord         `protobuf:"bytes,2,rep,name=records" json:"records,omitempty"`
	Mac           []byte                 `protobuf:"bytes,3,opt,name=mac" json:"mac,omitempty"`
	KeyId         *KeyId                 `protobuf:"bytes,4,opt,name=keyId" json:"keyId,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncdSnapshot) Reset() {
	*x = SyncdSnapshot{}
	mi := &file_chatwire_wire_proto_msgTypes[35]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdSnapshot) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdSnapshot) ProtoMessage() {}

func (x *SyncdSnapshot) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[35]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SyncdSnapshot) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{35}
}

func (x *SyncdSnapshot) GetVersion() *SyncdVersion {
	if x != nil {
		return x.Version
	}
	return nil
}

func (x *SyncdSnapshot) GetRecords() []*SyncdRecord {
	if x != nil {
		return x.Records
	}
	return nil
}

func (x *SyncdSnapshot) GetMac() []byte {
	if x != nil {
		return x.Mac
	}
	return nil
}

func (x *SyncdSnapshot) GetKeyId() *KeyId {
	if x != nil {
		return x.KeyId
	}
	return nil
}

type SyncdValue struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Blob          []byte                 `protobuf:"bytes,1,opt,name=blob" json:"blob,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncdValue) Reset() {
	*x = SyncdValue{}
	mi := &file_chatwire_wire_proto_msgTypes[36]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdValue) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdValue) ProtoMessage() {}

func (x *SyncdValue) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[36]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SyncdValue) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{36}
}

func (x *SyncdValue) GetBlob() []byte {
	if x != nil {
		return x.Blob
	}
	return nil
}

type SyncdVersion struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Version       *uint64                `protobuf:"varint,1,opt,name=version" json:"version,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncdVersion) Reset() {
	*x = SyncdVersion{}
	mi := &file_chatwire_wire_proto_msgTypes[37]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncdVersion) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncdVersion) ProtoMessage() {}

func (x *SyncdVersion) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[37]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SyncdVersion) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{37}
}

func (x *SyncdVersion) GetVersion() uint64 {
	if x != nil && x.Version != nil {
		return *x.Version
	}
	return 0
}

type ClientPayload_DevicePairingRegistrationData struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ClientPayload_DevicePairingRegistrationData) Reset() {
	*x = ClientPayload_DevicePairingRegistrationData{}
	mi := &file_chatwire_wire_proto_msgTypes[38]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClientPayload_DevicePairingRegistrationData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClientPayload_DevicePairingRegistrationData) ProtoMessage() {}

func (x *ClientPayload_DevicePairingRegistrationData) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[38]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ClientPayload_DevicePairingRegistrationData) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{4, 0}
}

type Message_AlbumMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_AlbumMessage) Reset() {
	*x = Message_AlbumMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[39]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_AlbumMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_AlbumMessage) ProtoMessage() {}

func (x *Message_AlbumMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[39]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_AlbumMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 0}
}

func (x *Message_AlbumMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_AppStateSyncKey struct {
	state         protoimpl.MessageState       `protogen:"open.v1"`
	KeyId         *Message_AppStateSyncKeyId   `protobuf:"bytes,1,opt,name=keyId" json:"keyId,omitempty"`
	KeyData       *Message_AppStateSyncKeyData `protobuf:"bytes,2,opt,name=keyData" json:"keyData,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_AppStateSyncKey) Reset() {
	*x = Message_AppStateSyncKey{}
	mi := &file_chatwire_wire_proto_msgTypes[40]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_AppStateSyncKey) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_AppStateSyncKey) ProtoMessage() {}

func (x *Message_AppStateSyncKey) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[40]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_AppStateSyncKey) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 1}
}

func (x *Message_AppStateSyncKey) GetKeyId() *Message_AppStateSyncKeyId {
	if x != nil {
		return x.KeyId
	}
	return nil
}

func (x *Message_AppStateSyncKey) GetKeyData() *Message_AppStateSyncKeyData {
	if x != nil {
		return x.KeyData
	}
	return nil
}

type Message_AppStateSyncKeyData struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	KeyData       []byte                 `protobuf:"bytes,1,opt,name=keyData" json:"keyData,omitempty"`
	Timestamp     *int64                 `protobuf:"varint,3,opt,name=timestamp" json:"timestamp,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_AppStateSyncKeyData) Reset() {
	*x = Message_AppStateSyncKeyData{}
	mi := &file_chatwire_wire_proto_msgTypes[41]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_AppStateSyncKeyData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_AppStateSyncKeyData) ProtoMessage() {}

func (x *Message_AppStateSyncKeyData) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[41]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_AppStateSyncKeyData) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 2}
}

func (x *Message_AppStateSyncKeyData) GetKeyData() []byte {
	if x != nil {
		return x.KeyData
	}
	return nil
}

func (x *Message_AppStateSyncKeyData) GetTimestamp() int64 {
	if x != nil && x.Timestamp != nil {
		return *x.Timestamp
	}
	return 0
}

type Message_AppStateSyncKeyId struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	KeyId         []byte                 `protobuf:"bytes,1,opt,name=keyId" json:"keyId,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_AppStateSyncKeyId) Reset() {
	*x = Message_AppStateSyncKeyId{}
	mi := &file_chatwire_wire_proto_msgTypes[42]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_AppStateSyncKeyId) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_AppStateSyncKeyId) ProtoMessage() {}

func (x *Message_AppStateSyncKeyId) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[42]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_AppStateSyncKeyId) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 3}
}

func (x *Message_AppStateSyncKeyId) GetKeyId() []byte {
	if x != nil {
		return x.KeyId
	}
	return nil
}

type Message_AppStateSyncKeyShare struct {
	state         protoimpl.MessageState     `protogen:"open.v1"`
	Keys          []*Message_AppStateSyncKey `protobuf:"bytes,1,rep,name=keys" json:"keys,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_AppStateSyncKeyShare) Reset() {
	*x = Message_AppStateSyncKeyShare{}
	mi := &file_chatwire_wire_proto_msgTypes[43]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_AppStateSyncKeyShare) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_AppStateSyncKeyShare) ProtoMessage() {}

func (x *Message_AppStateSyncKeyShare) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[43]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_AppStateSyncKeyShare) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 4}
}

func (x *Message_AppStateSyncKeyShare) GetKeys() []*Message_AppStateSyncKey {
	if x != nil {
		return x.Keys
	}
	return nil
}

type Message_AudioMessage struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	Url               *string                `protobuf:"bytes,1,opt,name=url" json:"url,omitempty"`
	Mimetype          *string                `protobuf:"bytes,2,opt,name=mimetype" json:"mimetype,omitempty"`
	FileSha256        []byte                 `protobuf:"bytes,3,opt,name=fileSha256" json:"fileSha256,omitempty"`
	FileLength        *uint64                `protobuf:"varint,4,opt,name=fileLength" json:"fileLength,omitempty"`
	Seconds           *uint32                `protobuf:"varint,5,opt,name=seconds" json:"seconds,omitempty"`
	Ptt               *bool                  `protobuf:"varint,6,opt,name=ptt" json:"ptt,omitempty"`
	MediaKey          []byte                 `protobuf:"bytes,7,opt,name=mediaKey" json:"mediaKey,omitempty"`
	FileEncSha256     []byte                 `protobuf:"bytes,8,opt,name=fileEncSha256" json:"fileEncSha256,omitempty"`
	DirectPath        *string                `protobuf:"bytes,9,opt,name=directPath" json:"directPath,omitempty"`
	MediaKeyTimestamp *int64                 `protobuf:"varint,10,opt,name=mediaKeyTimestamp" json:"mediaKeyTimestamp,omitempty"`
	ContextInfo       *ContextInfo           `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
	ViewOnce          *bool                  `protobuf:"varint,21,opt,name=viewOnce" json:"viewOnce,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *Message_AudioMessage) Reset() {
	*x = Message_AudioMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[44]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_AudioMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_AudioMessage) ProtoMessage() {}

func (x *Message_AudioMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[44]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_AudioMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 5}
}

func (x *Message_AudioMessage) GetUrl() string {
	if x != nil && x.Url != nil {
		return *x.Url
	}
	return ""
}

func (x *Message_AudioMessage) GetMimetype() string {
	if x != nil && x.Mimetype != nil {
		return *x.Mimetype
	}
	return ""
}

func (x *Message_AudioMessage) GetFileSha256() []byte {
	if x != nil {
		return x.FileSha256
	}
	return nil
}

func (x *Message_AudioMessage) GetFileLength() uint64 {
	if x != nil && x.FileLength != nil {
		return *x.FileLength
	}
	return 0
}

func (x *Message_AudioMessage) GetSeconds() uint32 {
	if x != nil && x.Seconds != nil {
		return *x.Seconds
	}
	return 0
}

func (x *Message_AudioMessage) GetPtt() bool {
	if x != nil && x.Ptt != nil {
		return *x.Ptt
	}
	return false
}

func (x *Message_AudioMessage) GetMediaKey() []byte {
	if x != nil {
		return x.MediaKey
	}
	return nil
}

func (x *Message_AudioMessage) GetFileEncSha256() []byte {
	if x != nil {
		return x.FileEncSha256
	}
	return nil
}

func (x *Message_AudioMessage) GetDirectPath() string {
	if x != nil && x.DirectPath != nil {
		return *x.DirectPath
	}
	return ""
}

func (x *Message_AudioMessage) GetMediaKeyTimestamp() int64 {
	if x != nil && x.MediaKeyTimestamp != nil {
		return *x.MediaKeyTimestamp
	}
	return 0
}

func (x *Message_AudioMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

func (x *Message_AudioMessage) GetViewOnce() bool {
	if x != nil && x.ViewOnce != nil {
		return *x.ViewOnce
	}
	return false
}

type Message_ButtonsMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,8,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_ButtonsMessage) Reset() {
	*x = Message_ButtonsMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[45]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_ButtonsMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_ButtonsMessage) ProtoMessage() {}

func (x *Message_ButtonsMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[45]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_ButtonsMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 6}
}

func (x *Message_ButtonsMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_ButtonsResponseMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,3,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_ButtonsResponseMessage) Reset() {
	*x = Message_ButtonsResponseMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[46]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_ButtonsResponseMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_ButtonsResponseMessage) ProtoMessage() {}

func (x *Message_ButtonsResponseMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[46]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_ButtonsResponseMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 7}
}

func (x *Message_ButtonsResponseMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_Call struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,7,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_Call) Reset() {
	*x = Message_Call{}
	mi := &file_chatwire_wire_proto_msgTypes[47]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_Call) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_Call) ProtoMessage() {}

func (x *Message_Call) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[47]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_Call) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 8}
}

func (x *Message_Call) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_ContactMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	DisplayName   *string                `protobuf:"bytes,1,opt,name=displayName" json:"displayName,omitempty"`
	Vcard         *string                `protobuf:"bytes,16,opt,name=vcard" json:"vcard,omitempty"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_ContactMessage) Reset() {
	*x = Message_ContactMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[48]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_ContactMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_ContactMessage) ProtoMessage() {}

func (x *Message_ContactMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[48]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_ContactMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 9}
}

func (x *Message_ContactMessage) GetDisplayName() string {
	if x != nil && x.DisplayName != nil {
		return *x.DisplayName
	}
	return ""
}

func (x *Message_ContactMessage) GetVcard() string {
	if x != nil && x.Vcard != nil {
		return *x.Vcard
	}
	return ""
}

func (x *Message_ContactMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_ContactsArrayMessage struct {
	state         protoimpl.MessageState    `protogen:"open.v1"`
	DisplayName   *string                   `protobuf:"bytes,1,opt,name=displayName" json:"displayName,omitempty"`
	Contacts      []*Message_ContactMessage `protobuf:"bytes,2,rep,name=contacts" json:"contacts,omitempty"`
	ContextInfo   *ContextInfo              `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_ContactsArrayMessage) Reset() {
	*x = Message_ContactsArrayMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[49]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_ContactsArrayMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_ContactsArrayMessage) ProtoMessage() {}

func (x *Message_ContactsArrayMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[49]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_ContactsArrayMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 10}
}

func (x *Message_ContactsArrayMessage) GetDisplayName() string {
	if x != nil && x.DisplayName != nil {
		return *x.DisplayName
	}
	return ""
}

func (x *Message_ContactsArrayMessage) GetContacts() []*Message_ContactMessage {
	if x != nil {
		return x.Contacts
	}
	return nil
}

func (x *Message_ContactsArrayMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_DeviceSentMessage struct {
	state          protoimpl.MessageState `protogen:"open.v1"`
	DestinationJid *string                `protobuf:"bytes,1,opt,name=destinationJid" json:"destinationJid,omitempty"`
	Message        *Message               `protobuf:"bytes,2,opt,name=message" json:"message,omitempty"`
	unknownFields  protoimpl.UnknownFields
	sizeCache      protoimpl.SizeCache
}

func (x *Message_DeviceSentMessage) Reset() {
	*x = Message_DeviceSentMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[50]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_DeviceSentMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_DeviceSentMessage) ProtoMessage() {}

func (x *Message_DeviceSentMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[50]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_DeviceSentMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 11}
}

func (x *Message_DeviceSentMessage) GetDestinationJid() string {
	if x != nil && x.DestinationJid != nil {
		return *x.DestinationJid
	}
	return ""
}

func (x *Message_DeviceSentMessage) GetMessage() *Message {
	if x != nil {
		return x.Message
	}
	return nil
}

type Message_DocumentMessage struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	Url               *string                `protobuf:"bytes,1,opt,name=url" json:"url,omitempty"`
	Mimetype          *string                `protobuf:"bytes,2,opt,name=mimetype" json:"mimetype,omitempty"`
	Title             *string                `protobuf:"bytes,3,opt,name=title" json:"title,omitempty"`
	FileSha256        []byte                 `protobuf:"bytes,4,opt,name=fileSha256" json:"fileSha256,omitempty"`
	FileLength        *uint64                `protobuf:"varint,5,opt,name=fileLength" json:"fileLength,omitempty"`
	MediaKey          []byte                 `protobuf:"bytes,7,opt,name=mediaKey" json:"mediaKey,omitempty"`
	FileName          *string                `protobuf:"bytes,8,opt,name=fileName" json:"fileName,omitempty"`
	FileEncSha256     []byte                 `protobuf:"bytes,9,opt,name=fileEncSha256" json:"fileEncSha256,omitempty"`
	DirectPath        *string                `protobuf:"bytes,10,opt,name=directPath" json:"directPath,omitempty"`
	MediaKeyTimestamp *int64                 `protobuf:"varint,11,opt,name=mediaKeyTimestamp" json:"mediaKeyTimestamp,omitempty"`
	ContextInfo       *ContextInfo           `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
	Caption           *string                `protobuf:"bytes,20,opt,name=caption" json:"caption,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *Message_DocumentMessage) Reset() {
	*x = Message_DocumentMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[51]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_DocumentMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_DocumentMessage) ProtoMessage() {}

func (x *Message_DocumentMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[51]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_DocumentMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 12}
}

func (x *Message_DocumentMessage) GetUrl() string {
	if x != nil && x.Url != nil {
		return *x.Url
	}
	return ""
}

func (x *Message_DocumentMessage) GetMimetype() string {
	if x != nil && x.Mimetype != nil {
		return *x.Mimetype
	}
	return ""
}

func (x *Message_DocumentMessage) GetTitle() string {
	if x != nil && x.Title != nil {
		return *x.Title
	}
	return ""
}

func (x *Message_DocumentMessage) GetFileSha256() []byte {
	if x != nil {
		return x.FileSha256
	}
	return nil
}

func (x *Message_DocumentMessage) GetFileLength() uint64 {
	if x != nil && x.FileLength != nil {
		return *x.FileLength
	}
	return 0
}

func (x *Message_DocumentMessage) GetMediaKey() []byte {
	if x != nil {
		return x.MediaKey
	}
	return nil
}

func (x *Message_DocumentMessage) GetFileName() string {
	if x != nil && x.FileName != nil {
		return *x.FileName
	}
	return ""
}

func (x *Message_DocumentMessage) GetFileEncSha256() []byte {
	if x != nil {
		return x.FileEncSha256
	}
	return nil
}

func (x *Message_DocumentMessage) GetDirectPath() string {
	if x != nil && x.DirectPath != nil {
		return *x.DirectPath
	}
	return ""
}

func (x *Message_DocumentMessage) GetMediaKeyTimestamp() int64 {
	if x != nil && x.MediaKeyTimestamp != nil {
		return *x.MediaKeyTimestamp
	}
	return 0
}

func (x *Message_DocumentMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

func (x *Message_DocumentMessage) GetCaption() string {
	if x != nil && x.Caption != nil {
		return *x.Caption
	}
	return ""
}

type Message_EncCommentMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_EncCommentMessage) Reset() {
	*x = Message_EncCommentMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[52]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_EncCommentMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_EncCommentMessage) ProtoMessage() {}

func (x *Message_EncCommentMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[52]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_EncCommentMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 13}
}

type Message_EncEventResponseMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_EncEventResponseMessage) Reset() {
	*x = Message_EncEventResponseMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[53]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_EncEventResponseMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_EncEventResponseMessage) ProtoMessage() {}

func (x *Message_EncEventResponseMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[53]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_EncEventResponseMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 14}
}

type Message_EncReactionMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_EncReactionMessage) Reset() {
	*x = Message_EncReactionMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[54]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_EncReactionMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_EncReactionMessage) ProtoMessage() {}

func (x *Message_EncReactionMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[54]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_EncReactionMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 15}
}

type Message_EventInviteMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,1,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_EventInviteMessage) Reset() {
	*x = Message_EventInviteMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[55]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_EventInviteMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_EventInviteMessage) ProtoMessage() {}

func (x *Message_EventInviteMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[55]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_EventInviteMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 16}
}

func (x *Message_EventInviteMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_EventMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,1,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_EventMessage) Reset() {
	*x = Message_EventMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[56]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_EventMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_EventMessage) ProtoMessage() {}

func (x *Message_EventMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[56]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_EventMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 17}
}

func (x *Message_EventMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_ExtendedTextMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Text          *string                `protobuf:"bytes,1,opt,name=text" json:"text,omitempty"`
	MatchedText   *string                `protobuf:"bytes,2,opt,name=matchedText" json:"matchedText,omitempty"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_ExtendedTextMessage) Reset() {
	*x = Message_ExtendedTextMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[57]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_ExtendedTextMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_ExtendedTextMessage) ProtoMessage() {}

func (x *Message_ExtendedTextMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[57]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_ExtendedTextMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 18}
}

func (x *Message_ExtendedTextMessage) GetText() string {
	if x != nil && x.Text != nil {
		return *x.Text
	}
	return ""
}

func (x *Message_ExtendedTextMessage) GetMatchedText() string {
	if x != nil && x.MatchedText != nil {
		return *x.MatchedText
	}
	return ""
}

func (x *Message_ExtendedTextMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_FutureProofMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Message       *Message               `protobuf:"bytes,1,opt,name=message" json:"message,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_FutureProofMessage) Reset() {
	*x = Message_FutureProofMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[58]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_FutureProofMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_FutureProofMessage) ProtoMessage() {}

func (x *Message_FutureProofMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[58]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_FutureProofMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 19}
}

func (x *Message_FutureProofMessage) GetMessage() *Message {
	if x != nil {
		return x.Message
	}
	return nil
}

type Message_GroupInviteMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,7,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_GroupInviteMessage) Reset() {
	*x = Message_GroupInviteMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[59]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_GroupInviteMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_GroupInviteMessage) ProtoMessage() {}

func (x *Message_GroupInviteMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[59]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_GroupInviteMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 20}
}

func (x *Message_GroupInviteMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_HistorySyncNotification struct {
	state                             protoimpl.MessageState   `protogen:"open.v1"`
	FileSha256                        []byte                   `protobuf:"bytes,1,opt,name=fileSha256" json:"fileSha256,omitempty"`
	FileLength                        *uint64                  `protobuf:"varint,2,opt,name=fileLength" json:"fileLength,omitempty"`
	MediaKey                          []byte                   `protobuf:"bytes,3,opt,name=mediaKey" json:"mediaKey,omitempty"`
	FileEncSha256                     []byte                   `protobuf:"bytes,4,opt,name=fileEncSha256" json:"fileEncSha256,omitempty"`
	DirectPath                        *string                  `protobuf:"bytes,5,opt,name=directPath" json:"directPath,omitempty"`
	SyncType                          *Message_HistorySyncType `protobuf:"varint,6,opt,name=syncType,enum=chatwire.wire.Message_HistorySyncType" json:"syncType,omitempty"`
	InitialHistBootstrapInlinePayload []byte                   `protobuf:"bytes,11,opt,name=initialHistBootstrapInlinePayload" json:"initialHistBootstrapInlinePayload,omitempty"`
	unknownFields                     protoimpl.UnknownFields
	sizeCache                         protoimpl.SizeCache
}

func (x *Message_HistorySyncNotification) Reset() {
	*x = Message_HistorySyncNotification{}
	mi := &file_chatwire_wire_proto_msgTypes[60]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_HistorySyncNotification) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_HistorySyncNotification) ProtoMessage() {}

func (x *Message_HistorySyncNotification) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[60]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_HistorySyncNotification) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 21}
}

func (x *Message_HistorySyncNotification) GetFileSha256() []byte {
	if x != nil {
		return x.FileSha256
	}
	return nil
}

func (x *Message_HistorySyncNotification) GetFileLength() uint64 {
	if x != nil && x.FileLength != nil {
		return *x.FileLength
	}
	return 0
}

func (x *Message_HistorySyncNotification) GetMediaKey() []byte {
	if x != nil {
		return x.MediaKey
	}
	return nil
}

func (x *Message_HistorySyncNotification) GetFileEncSha256() []byte {
	if x != nil {
		return x.FileEncSha256
	}
	return nil
}

func (x *Message_HistorySyncNotification) GetDirectPath() string {
	if x != nil && x.DirectPath != nil {
		return *x.DirectPath
	}
	return ""
}

func (x *Message_HistorySyncNotification) GetSyncType() Message_HistorySyncType {
	if x != nil && x.SyncType != nil {
		return *x.SyncType
	}
	return Message_INITIAL_BOOTSTRAP
}

func (x *Message_HistorySyncNotification) GetInitialHistBootstrapInlinePayload() []byte {
	if x != nil {
		return x.InitialHistBootstrapInlinePayload
	}
	return nil
}

type Message_ImageMessage struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	Url               *string                `protobuf:"bytes,1,opt,name=url" json:"url,omitempty"`
	Mimetype          *string                `protobuf:"bytes,2,opt,name=mimetype" json:"mimetype,omitempty"`
	Caption           *string                `protobuf:"bytes,3,opt,name=caption" json:"caption,omitempty"`
	FileSha256        []byte                 `protobuf:"bytes,4,opt,name=fileSha256" json:"fileSha256,omitempty"`
	FileLength        *uint64                `protobuf:"varint,5,opt,name=fileLength" json:"fileLength,omitempty"`
	Height            *uint32                `protobuf:"varint,6,opt,name=height" json:"height,omitempty"`
	Width             *uint32                `protobuf:"varint,7,opt,name=width" json:"width,omitempty"`
	MediaKey          []byte                 `protobuf:"bytes,8,opt,name=mediaKey" json:"mediaKey,omitempty"`
	FileEncSha256     []byte                 `protobuf:"bytes,9,opt,name=fileEncSha256" json:"fileEncSha256,omitempty"`
	DirectPath        *string                `protobuf:"bytes,11,opt,name=directPath" json:"directPath,omitempty"`
	MediaKeyTimestamp *int64                 `protobuf:"varint,12,opt,name=mediaKeyTimestamp" json:"mediaKeyTimestamp,omitempty"`
	JpegThumbnail     []byte                 `protobuf:"bytes,16,opt,name=jpegThumbnail" json:"jpegThumbnail,omitempty"`
	ContextInfo       *ContextInfo           `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
	ViewOnce          *bool                  `protobuf:"varint,25,opt,name=viewOnce" json:"viewOnce,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *Message_ImageMessage) Reset() {
	*x = Message_ImageMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[61]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_ImageMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_ImageMessage) ProtoMessage() {}

func (x *Message_ImageMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[61]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_ImageMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 22}
}

func (x *Message_ImageMessage) GetUrl() string {
	if x != nil && x.Url != nil {
		return *x.Url
	}
	return ""
}

func (x *Message_ImageMessage) GetMimetype() string {
	if x != nil && x.Mimetype != nil {
		return *x.Mimetype
	}
	return ""
}

func (x *Message_ImageMessage) GetCaption() string {
	if x != nil && x.Caption != nil {
		return *x.Caption
	}
	return ""
}

func (x *Message_ImageMessage) GetFileSha256() []byte {
	if x != nil {
		return x.FileSha256
	}
	return nil
}

func (x *Message_ImageMessage) GetFileLength() uint64 {
	if x != nil && x.FileLength != nil {
		return *x.FileLength
	}
	return 0
}

func (x *Message_ImageMessage) GetHeight() uint32 {
	if x != nil && x.Height != nil {
		return *x.Height
	}
	return 0
}

func (x *Message_ImageMessage) GetWidth() uint32 {
	if x != nil && x.Width != nil {
		return *x.Width
	}
	return 0
}

func (x *Message_ImageMessage) GetMediaKey() []byte {
	if x != nil {
		return x.MediaKey
	}
	return nil
}

func (x *Message_ImageMessage) GetFileEncSha256() []byte {
	if x != nil {
		return x.FileEncSha256
	}
	return nil
}

func (x *Message_ImageMessage) GetDirectPath() string {
	if x != nil && x.DirectPath != nil {
		return *x.DirectPath
	}
	return ""
}

func (x *Message_ImageMessage) GetMediaKeyTimestamp() int64 {
	if x != nil && x.MediaKeyTimestamp != nil {
		return *x.MediaKeyTimestamp
	}
	return 0
}

func (x *Message_ImageMessage) GetJpegThumbnail() []byte {
	if x != nil {
		return x.JpegThumbnail
	}
	return nil
}

func (x *Message_ImageMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

func (x *Message_ImageMessage) GetViewOnce() bool {
	if x != nil && x.ViewOnce != nil {
		return *x.ViewOnce
	}
	return false
}

type Message_InteractiveMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,15,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_InteractiveMessage) Reset() {
	*x = Message_InteractiveMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[62]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_InteractiveMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_InteractiveMessage) ProtoMessage() {}

func (x *Message_InteractiveMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[62]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_InteractiveMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 23}
}

func (x *Message_InteractiveMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_InteractiveResponseMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,15,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_InteractiveResponseMessage) Reset() {
	*x = Message_InteractiveResponseMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[63]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_InteractiveResponseMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_InteractiveResponseMessage) ProtoMessage() {}

func (x *Message_InteractiveResponseMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[63]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_InteractiveResponseMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 24}
}

func (x *Message_InteractiveResponseMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_KeepInChatMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_KeepInChatMessage) Reset() {
	*x = Message_KeepInChatMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[64]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_KeepInChatMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_KeepInChatMessage) ProtoMessage() {}

func (x *Message_KeepInChatMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[64]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_KeepInChatMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 25}
}

type Message_ListMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,8,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_ListMessage) Reset() {
	*x = Message_ListMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[65]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_ListMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_ListMessage) ProtoMessage() {}

func (x *Message_ListMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[65]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_ListMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 26}
}

func (x *Message_ListMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_ListResponseMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,4,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_ListResponseMessage) Reset() {
	*x = Message_ListResponseMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[66]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_ListResponseMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_ListResponseMessage) ProtoMessage() {}

func (x *Message_ListResponseMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[66]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_ListResponseMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 27}
}

func (x *Message_ListResponseMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_LiveLocationMessage struct {
	state            protoimpl.MessageState `protogen:"open.v1"`
	DegreesLatitude  *float64               `protobuf:"fixed64,1,opt,name=degreesLatitude" json:"degreesLatitude,omitempty"`
	DegreesLongitude *float64               `protobuf:"fixed64,2,opt,name=degreesLongitude" json:"degreesLongitude,omitempty"`
	Caption          *string                `protobuf:"bytes,6,opt,name=caption" json:"caption,omitempty"`
	ContextInfo      *ContextInfo           `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields    protoimpl.UnknownFields
	sizeCache        protoimpl.SizeCache
}

func (x *Message_LiveLocationMessage) Reset() {
	*x = Message_LiveLocationMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[67]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_LiveLocationMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_LiveLocationMessage) ProtoMessage() {}

func (x *Message_LiveLocationMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[67]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_LiveLocationMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 28}
}

func (x *Message_LiveLocationMessage) GetDegreesLatitude() float64 {
	if x != nil && x.DegreesLatitude != nil {
		return *x.DegreesLatitude
	}
	return 0
}

func (x *Message_LiveLocationMessage) GetDegreesLongitude() float64 {
	if x != nil && x.DegreesLongitude != nil {
		return *x.DegreesLongitude
	}
	return 0
}

func (x *Message_LiveLocationMessage) GetCaption() string {
	if x != nil && x.Caption != nil {
		return *x.Caption
	}
	return ""
}

func (x *Message_LiveLocationMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_LocationMessage struct {
	state            protoimpl.MessageState `protogen:"open.v1"`
	DegreesLatitude  *float64               `protobuf:"fixed64,1,opt,name=degreesLatitude" json:"degreesLatitude,omitempty"`
	DegreesLongitude *float64               `protobuf:"fixed64,2,opt,name=degreesLongitude" json:"degreesLongitude,omitempty"`
	Name             *string                `protobuf:"bytes,3,opt,name=name" json:"name,omitempty"`
	Address          *string                `protobuf:"bytes,4,opt,name=address" json:"address,omitempty"`
	Url              *string                `protobuf:"bytes,5,opt,name=url" json:"url,omitempty"`
	IsLive           *bool                  `protobuf:"varint,6,opt,name=isLive" json:"isLive,omitempty"`
	Comment          *string                `protobuf:"bytes,11,opt,name=comment" json:"comment,omitempty"`
	ContextInfo      *ContextInfo           `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields    protoimpl.UnknownFields
	sizeCache        protoimpl.SizeCache
}

func (x *Message_LocationMessage) Reset() {
	*x = Message_LocationMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[68]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_LocationMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_LocationMessage) ProtoMessage() {}

func (x *Message_LocationMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[68]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_LocationMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 29}
}

func (x *Message_LocationMessage) GetDegreesLatitude() float64 {
	if x != nil && x.DegreesLatitude != nil {
		return *x.DegreesLatitude
	}
	return 0
}

func (x *Message_LocationMessage) GetDegreesLongitude() float64 {
	if x != nil && x.DegreesLongitude != nil {
		return *x.DegreesLongitude
	}
	return 0
}

func (x *Message_LocationMessage) GetName() string {
	if x != nil && x.Name != nil {
		return *x.Name
	}
	return ""
}

func (x *Message_LocationMessage) GetAddress() string {
	if x != nil && x.Address != nil {
		return *x.Address
	}
	return ""
}

func (x *Message_LocationMessage) GetUrl() string {
	if x != nil && x.Url != nil {
		return *x.Url
	}
	return ""
}

func (x *Message_LocationMessage) GetIsLive() bool {
	if x != nil && x.IsLive != nil {
		return *x.IsLive
	}
	return false
}

func (x *Message_LocationMessage) GetComment() string {
	if x != nil && x.Comment != nil {
		return *x.Comment
	}
	return ""
}

func (x *Message_LocationMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_MessageHistoryBundle struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	FileSha256        []byte                 `protobuf:"bytes,2,opt,name=fileSha256" json:"fileSha256,omitempty"`
	MediaKey          []byte                 `protobuf:"bytes,3,opt,name=mediaKey" json:"mediaKey,omitempty"`
	FileEncSha256     []byte                 `protobuf:"bytes,4,opt,name=fileEncSha256" json:"fileEncSha256,omitempty"`
	DirectPath        *string                `protobuf:"bytes,5,opt,name=directPath" json:"directPath,omitempty"`
	MediaKeyTimestamp *int64                 `protobuf:"varint,6,opt,name=mediaKeyTimestamp" json:"mediaKeyTimestamp,omitempty"`
	ContextInfo       *ContextInfo           `protobuf:"bytes,7,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *Message_MessageHistoryBundle) Reset() {
	*x = Message_MessageHistoryBundle{}
	mi := &file_chatwire_wire_proto_msgTypes[69]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_MessageHistoryBundle) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_MessageHistoryBundle) ProtoMessage() {}

func (x *Message_MessageHistoryBundle) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[69]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_MessageHistoryBundle) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 30}
}

func (x *Message_MessageHistoryBundle) GetFileSha256() []byte {
	if x != nil {
		return x.FileSha256
	}
	return nil
}

func (x *Message_MessageHistoryBundle) GetMediaKey() []byte {
	if x != nil {
		return x.MediaKey
	}
	return nil
}

func (x *Message_MessageHistoryBundle) GetFileEncSha256() []byte {
	if x != nil {
		return x.FileEncSha256
	}
	return nil
}

func (x *Message_MessageHistoryBundle) GetDirectPath() string {
	if x != nil && x.DirectPath != nil {
		return *x.DirectPath
	}
	return ""
}

func (x *Message_MessageHistoryBundle) GetMediaKeyTimestamp() int64 {
	if x != nil && x.MediaKeyTimestamp != nil {
		return *x.MediaKeyTimestamp
	}
	return 0
}

func (x *Message_MessageHistoryBundle) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_MessageHistoryNotice struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,1,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_MessageHistoryNotice) Reset() {
	*x = Message_MessageHistoryNotice{}
	mi := &file_chatwire_wire_proto_msgTypes[70]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_MessageHistoryNotice) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_MessageHistoryNotice) ProtoMessage() {}

func (x *Message_MessageHistoryNotice) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[70]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_MessageHistoryNotice) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 31}
}

func (x *Message_MessageHistoryNotice) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_MusicMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,5,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_MusicMessage) Reset() {
	*x = Message_MusicMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[71]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_MusicMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_MusicMessage) ProtoMessage() {}

func (x *Message_MusicMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[71]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_MusicMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 32}
}

func (x *Message_MusicMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_NewsletterAdminInviteMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,6,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_NewsletterAdminInviteMessage) Reset() {
	*x = Message_NewsletterAdminInviteMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[72]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_NewsletterAdminInviteMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_NewsletterAdminInviteMessage) ProtoMessage() {}

func (x *Message_NewsletterAdminInviteMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[72]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_NewsletterAdminInviteMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 33}
}

func (x *Message_NewsletterAdminInviteMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_NewsletterFollowerInviteMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,5,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_NewsletterFollowerInviteMessage) Reset() {
	*x = Message_NewsletterFollowerInviteMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[73]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_NewsletterFollowerInviteMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_NewsletterFollowerInviteMessage) ProtoMessage() {}

func (x *Message_NewsletterFollowerInviteMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[73]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_NewsletterFollowerInviteMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 34}
}

func (x *Message_NewsletterFollowerInviteMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_OrderMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_OrderMessage) Reset() {
	*x = Message_OrderMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[74]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_OrderMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_OrderMessage) ProtoMessage() {}

func (x *Message_OrderMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[74]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_OrderMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 35}
}

func (x *Message_OrderMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_PinInChatMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_PinInChatMessage) Reset() {
	*x = Message_PinInChatMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[75]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_PinInChatMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_PinInChatMessage) ProtoMessage() {}

func (x *Message_PinInChatMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[75]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_PinInChatMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 36}
}

type Message_PollCreationMessage struct {
	state                  protoimpl.MessageState                `protogen:"open.v1"`
	Name                   *string                               `protobuf:"bytes,2,opt,name=name" json:"name,omitempty"`
	Options                []*Message_PollCreationMessage_Option `protobuf:"bytes,3,rep,name=options" json:"options,omitempty"`
	SelectableOptionsCount *uint32                               `protobuf:"varint,4,opt,name=selectableOptionsCount" json:"selectableOptionsCount,omitempty"`
	ContextInfo            *ContextInfo                          `protobuf:"bytes,5,opt,name=contextInfo" json:"contextInfo,omitempty"`
	PollContentType        *Message_PollContentType              `protobuf:"varint,6,opt,name=pollContentType,enum=chatwire.wire.Message_PollContentType" json:"pollContentType,omitempty"`
	PollType               *Message_PollType                     `protobuf:"varint,7,opt,name=pollType,enum=chatwire.wire.Message_PollType" json:"pollType,omitempty"`
	unknownFields          protoimpl.UnknownFields
	sizeCache              protoimpl.SizeCache
}

func (x *Message_PollCreationMessage) Reset() {
	*x = Message_PollCreationMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[76]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_PollCreationMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_PollCreationMessage) ProtoMessage() {}

func (x *Message_PollCreationMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[76]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_PollCreationMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 37}
}

func (x *Message_PollCreationMessage) GetName() string {
	if x != nil && x.Name != nil {
		return *x.Name
	}
	return ""
}

func (x *Message_PollCreationMessage) GetOptions() []*Message_PollCreationMessage_Option {
	if x != nil {
		return x.Options
	}
	return nil
}

func (x *Message_PollCreationMessage) GetSelectableOptionsCount() uint32 {
	if x != nil && x.SelectableOptionsCount != nil {
		return *x.SelectableOptionsCount
	}
	return 0
}

func (x *Message_PollCreationMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

func (x *Message_PollCreationMessage) GetPollContentType() Message_PollContentType {
	if x != nil && x.PollContentType != nil {
		return *x.PollContentType
	}
	return Message_UNKNOWN
}

func (x *Message_PollCreationMessage) GetPollType() Message_PollType {
	if x != nil && x.PollType != nil {
		return *x.PollType
	}
	return Message_POLL
}

type Message_PollEncValue struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	EncPayload    []byte                 `protobuf:"bytes,1,opt,name=encPayload" json:"encPayload,omitempty"`
	EncIv         []byte                 `protobuf:"bytes,2,opt,name=encIv" json:"encIv,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_PollEncValue) Reset() {
	*x = Message_PollEncValue{}
	mi := &file_chatwire_wire_proto_msgTypes[77]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_PollEncValue) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_PollEncValue) ProtoMessage() {}

func (x *Message_PollEncValue) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[77]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_PollEncValue) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 38}
}

func (x *Message_PollEncValue) GetEncPayload() []byte {
	if x != nil {
		return x.EncPayload
	}
	return nil
}

func (x *Message_PollEncValue) GetEncIv() []byte {
	if x != nil {
		return x.EncIv
	}
	return nil
}

type Message_PollResultSnapshotMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,3,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_PollResultSnapshotMessage) Reset() {
	*x = Message_PollResultSnapshotMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[78]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_PollResultSnapshotMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_PollResultSnapshotMessage) ProtoMessage() {}

func (x *Message_PollResultSnapshotMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[78]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_PollResultSnapshotMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 39}
}

func (x *Message_PollResultSnapshotMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_PollUpdateMessage struct {
	state                  protoimpl.MessageState `protogen:"open.v1"`
	PollCreationMessageKey *MessageKey            `protobuf:"bytes,1,opt,name=pollCreationMessageKey" json:"pollCreationMessageKey,omitempty"`
	Vote                   *Message_PollEncValue  `protobuf:"bytes,2,opt,name=vote" json:"vote,omitempty"`
	SenderTimestampMs      *int64                 `protobuf:"varint,4,opt,name=senderTimestampMs" json:"senderTimestampMs,omitempty"`
	unknownFields          protoimpl.UnknownFields
	sizeCache              protoimpl.SizeCache
}

func (x *Message_PollUpdateMessage) Reset() {
	*x = Message_PollUpdateMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[79]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_PollUpdateMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_PollUpdateMessage) ProtoMessage() {}

func (x *Message_PollUpdateMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[79]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_PollUpdateMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 40}
}

func (x *Message_PollUpdateMessage) GetPollCreationMessageKey() *MessageKey {
	if x != nil {
		return x.PollCreationMessageKey
	}
	return nil
}

func (x *Message_PollUpdateMessage) GetVote() *Message_PollEncValue {
	if x != nil {
		return x.Vote
	}
	return nil
}

func (x *Message_PollUpdateMessage) GetSenderTimestampMs() int64 {
	if x != nil && x.SenderTimestampMs != nil {
		return *x.SenderTimestampMs
	}
	return 0
}

type Message_PollVoteMessage struct {
	state           protoimpl.MessageState `protogen:"open.v1"`
	SelectedOptions [][]byte               `protobuf:"bytes,1,rep,name=selectedOptions" json:"selectedOptions,omitempty"`
	unknownFields   protoimpl.UnknownFields
	sizeCache       protoimpl.SizeCache
}

func (x *Message_PollVoteMessage) Reset() {
	*x = Message_PollVoteMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[80]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_PollVoteMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_PollVoteMessage) ProtoMessage() {}

func (x *Message_PollVoteMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[80]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_PollVoteMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 41}
}

func (x *Message_PollVoteMessage) GetSelectedOptions() [][]byte {
	if x != nil {
		return x.SelectedOptions
	}
	return nil
}

type Message_ProductMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_ProductMessage) Reset() {
	*x = Message_ProductMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[81]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_ProductMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_ProductMessage) ProtoMessage() {}

func (x *Message_ProductMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[81]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_ProductMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 42}
}

func (x *Message_ProductMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_ProtocolMessage struct {
	state                   protoimpl.MessageState           `protogen:"open.v1"`
	Key                     *MessageKey                      `protobuf:"bytes,1,opt,name=key" json:"key,omitempty"`
	Type                    *Message_ProtocolMessage_Type    `protobuf:"varint,2,opt,name=type,enum=chatwire.wire.Message_ProtocolMessage_Type" json:"type,omitempty"`
	HistorySyncNotification *Message_HistorySyncNotification `protobuf:"bytes,6,opt,name=historySyncNotification" json:"historySyncNotification,omitempty"`
	AppStateSyncKeyShare    *Message_AppStateSyncKeyShare    `protobuf:"bytes,7,opt,name=appStateSyncKeyShare" json:"appStateSyncKeyShare,omitempty"`
	EditedMessage           *Message                         `protobuf:"bytes,14,opt,name=editedMessage" json:"editedMessage,omitempty"`
	TimestampMs             *int64                           `protobuf:"varint,15,opt,name=timestampMs" json:"timestampMs,omitempty"`
	unknownFields           protoimpl.UnknownFields
	sizeCache               protoimpl.SizeCache
}

func (x *Message_ProtocolMessage) Reset() {
	*x = Message_ProtocolMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[82]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_ProtocolMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_ProtocolMessage) ProtoMessage() {}

func (x *Message_ProtocolMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[82]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_ProtocolMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 43}
}

func (x *Message_ProtocolMessage) GetKey() *MessageKey {
	if x != nil {
		return x.Key
	}
	return nil
}

func (x *Message_ProtocolMessage) GetType() Message_ProtocolMessage_Type {
	if x != nil && x.Type != nil {
		return *x.Type
	}
	return Message_ProtocolMessage_REVOKE
}

func (x *Message_ProtocolMessage) GetHistorySyncNotification() *Message_HistorySyncNotification {
	if x != nil {
		return x.HistorySyncNotification
	}
	return nil
}

func (x *Message_ProtocolMessage) GetAppStateSyncKeyShare() *Message_AppStateSyncKeyShare {
	if x != nil {
		return x.AppStateSyncKeyShare
	}
	return nil
}

func (x *Message_ProtocolMessage) GetEditedMessage() *Message {
	if x != nil {
		return x.EditedMessage
	}
	return nil
}

func (x *Message_ProtocolMessage) GetTimestampMs() int64 {
	if x != nil && x.TimestampMs != nil {
		return *x.TimestampMs
	}
	return 0
}

type Message_ReactionMessage struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	Key               *MessageKey            `protobuf:"bytes,1,opt,name=key" json:"key,omitempty"`
	Text              *string                `protobuf:"bytes,2,opt,name=text" json:"text,omitempty"`
	SenderTimestampMs *int64                 `protobuf:"varint,4,opt,name=senderTimestampMs" json:"senderTimestampMs,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *Message_ReactionMessage) Reset() {
	*x = Message_ReactionMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[83]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_ReactionMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_ReactionMessage) ProtoMessage() {}

func (x *Message_ReactionMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[83]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_ReactionMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 44}
}

func (x *Message_ReactionMessage) GetKey() *MessageKey {
	if x != nil {
		return x.Key
	}
	return nil
}

func (x *Message_ReactionMessage) GetText() string {
	if x != nil && x.Text != nil {
		return *x.Text
	}
	return ""
}

func (x *Message_ReactionMessage) GetSenderTimestampMs() int64 {
	if x != nil && x.SenderTimestampMs != nil {
		return *x.SenderTimestampMs
	}
	return 0
}

type Message_RequestPhoneNumberMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,1,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_RequestPhoneNumberMessage) Reset() {
	*x = Message_RequestPhoneNumberMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[84]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_RequestPhoneNumberMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_RequestPhoneNumberMessage) ProtoMessage() {}

func (x *Message_RequestPhoneNumberMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[84]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_RequestPhoneNumberMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 45}
}

func (x *Message_RequestPhoneNumberMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_SecretEncryptedMessage struct {
	state         protoimpl.MessageState                        `protogen:"open.v1"`
	SecretEncType *Message_SecretEncryptedMessage_SecretEncType `protobuf:"varint,4,opt,name=secretEncType,enum=chatwire.wire.Message_SecretEncryptedMessage_SecretEncType" json:"secretEncType,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_SecretEncryptedMessage) Reset() {
	*x = Message_SecretEncryptedMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[85]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_SecretEncryptedMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_SecretEncryptedMessage) ProtoMessage() {}

func (x *Message_SecretEncryptedMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[85]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_SecretEncryptedMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 46}
}

func (x *Message_SecretEncryptedMessage) GetSecretEncType() Message_SecretEncryptedMessage_SecretEncType {
	if x != nil && x.SecretEncType != nil {
		return *x.SecretEncType
	}
	return Message_SecretEncryptedMessage_UNKNOWN
}

type Message_SenderKeyDistributionMessage struct {
	state                               protoimpl.MessageState `protogen:"open.v1"`
	GroupId                             *string                `protobuf:"bytes,1,opt,name=groupId" json:"groupId,omitempty"`
	AxolotlSenderKeyDistributionMessage []byte                 `protobuf:"bytes,2,opt,name=axolotlSenderKeyDistributionMessage" json:"axolotlSenderKeyDistributionMessage,omitempty"`
	unknownFields                       protoimpl.UnknownFields
	sizeCache                           protoimpl.SizeCache
}

func (x *Message_SenderKeyDistributionMessage) Reset() {
	*x = Message_SenderKeyDistributionMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[86]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_SenderKeyDistributionMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_SenderKeyDistributionMessage) ProtoMessage() {}

func (x *Message_SenderKeyDistributionMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[86]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_SenderKeyDistributionMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 47}
}

func (x *Message_SenderKeyDistributionMessage) GetGroupId() string {
	if x != nil && x.GroupId != nil {
		return *x.GroupId
	}
	return ""
}

func (x *Message_SenderKeyDistributionMessage) GetAxolotlSenderKeyDistributionMessage() []byte {
	if x != nil {
		return x.AxolotlSenderKeyDistributionMessage
	}
	return nil
}

type Message_SplitPaymentMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_SplitPaymentMessage) Reset() {
	*x = Message_SplitPaymentMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[87]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_SplitPaymentMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_SplitPaymentMessage) ProtoMessage() {}

func (x *Message_SplitPaymentMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[87]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_SplitPaymentMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 48}
}

func (x *Message_SplitPaymentMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_StickerMessage struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	Url               *string                `protobuf:"bytes,1,opt,name=url" json:"url,omitempty"`
	FileSha256        []byte                 `protobuf:"bytes,2,opt,name=fileSha256" json:"fileSha256,omitempty"`
	FileEncSha256     []byte                 `protobuf:"bytes,3,opt,name=fileEncSha256" json:"fileEncSha256,omitempty"`
	MediaKey          []byte                 `protobuf:"bytes,4,opt,name=mediaKey" json:"mediaKey,omitempty"`
	Mimetype          *string                `protobuf:"bytes,5,opt,name=mimetype" json:"mimetype,omitempty"`
	DirectPath        *string                `protobuf:"bytes,8,opt,name=directPath" json:"directPath,omitempty"`
	FileLength        *uint64                `protobuf:"varint,9,opt,name=fileLength" json:"fileLength,omitempty"`
	MediaKeyTimestamp *int64                 `protobuf:"varint,10,opt,name=mediaKeyTimestamp" json:"mediaKeyTimestamp,omitempty"`
	ContextInfo       *ContextInfo           `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *Message_StickerMessage) Reset() {
	*x = Message_StickerMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[88]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_StickerMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_StickerMessage) ProtoMessage() {}

func (x *Message_StickerMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[88]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_StickerMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 49}
}

func (x *Message_StickerMessage) GetUrl() string {
	if x != nil && x.Url != nil {
		return *x.Url
	}
	return ""
}

func (x *Message_StickerMessage) GetFileSha256() []byte {
	if x != nil {
		return x.FileSha256
	}
	return nil
}

func (x *Message_StickerMessage) GetFileEncSha256() []byte {
	if x != nil {
		return x.FileEncSha256
	}
	return nil
}

func (x *Message_StickerMessage) GetMediaKey() []byte {
	if x != nil {
		return x.MediaKey
	}
	return nil
}

func (x *Message_StickerMessage) GetMimetype() string {
	if x != nil && x.Mimetype != nil {
		return *x.Mimetype
	}
	return ""
}

func (x *Message_StickerMessage) GetDirectPath() string {
	if x != nil && x.DirectPath != nil {
		return *x.DirectPath
	}
	return ""
}

func (x *Message_StickerMessage) GetFileLength() uint64 {
	if x != nil && x.FileLength != nil {
		return *x.FileLength
	}
	return 0
}

func (x *Message_StickerMessage) GetMediaKeyTimestamp() int64 {
	if x != nil && x.MediaKeyTimestamp != nil {
		return *x.MediaKeyTimestamp
	}
	return 0
}

func (x *Message_StickerMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_StickerPackMessage struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	FileLength        *uint64                `protobuf:"varint,5,opt,name=fileLength" json:"fileLength,omitempty"`
	FileSha256        []byte                 `protobuf:"bytes,6,opt,name=fileSha256" json:"fileSha256,omitempty"`
	FileEncSha256     []byte                 `protobuf:"bytes,7,opt,name=fileEncSha256" json:"fileEncSha256,omitempty"`
	MediaKey          []byte                 `protobuf:"bytes,8,opt,name=mediaKey" json:"mediaKey,omitempty"`
	DirectPath        *string                `protobuf:"bytes,9,opt,name=directPath" json:"directPath,omitempty"`
	ContextInfo       *ContextInfo           `protobuf:"bytes,11,opt,name=contextInfo" json:"contextInfo,omitempty"`
	MediaKeyTimestamp *int64                 `protobuf:"varint,13,opt,name=mediaKeyTimestamp" json:"mediaKeyTimestamp,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *Message_StickerPackMessage) Reset() {
	*x = Message_StickerPackMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[89]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_StickerPackMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_StickerPackMessage) ProtoMessage() {}

func (x *Message_StickerPackMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[89]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_StickerPackMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 50}
}

func (x *Message_StickerPackMessage) GetFileLength() uint64 {
	if x != nil && x.FileLength != nil {
		return *x.FileLength
	}
	return 0
}

func (x *Message_StickerPackMessage) GetFileSha256() []byte {
	if x != nil {
		return x.FileSha256
	}
	return nil
}

func (x *Message_StickerPackMessage) GetFileEncSha256() []byte {
	if x != nil {
		return x.FileEncSha256
	}
	return nil
}

func (x *Message_StickerPackMessage) GetMediaKey() []byte {
	if x != nil {
		return x.MediaKey
	}
	return nil
}

func (x *Message_StickerPackMessage) GetDirectPath() string {
	if x != nil && x.DirectPath != nil {
		return *x.DirectPath
	}
	return ""
}

func (x *Message_StickerPackMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

func (x *Message_StickerPackMessage) GetMediaKeyTimestamp() int64 {
	if x != nil && x.MediaKeyTimestamp != nil {
		return *x.MediaKeyTimestamp
	}
	return 0
}

type Message_TemplateButtonReplyMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,3,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_TemplateButtonReplyMessage) Reset() {
	*x = Message_TemplateButtonReplyMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[90]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_TemplateButtonReplyMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_TemplateButtonReplyMessage) ProtoMessage() {}

func (x *Message_TemplateButtonReplyMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[90]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_TemplateButtonReplyMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 51}
}

func (x *Message_TemplateButtonReplyMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_TemplateMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ContextInfo   *ContextInfo           `protobuf:"bytes,3,opt,name=contextInfo" json:"contextInfo,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_TemplateMessage) Reset() {
	*x = Message_TemplateMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[91]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_TemplateMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_TemplateMessage) ProtoMessage() {}

func (x *Message_TemplateMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[91]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_TemplateMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 52}
}

func (x *Message_TemplateMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

type Message_VideoMessage struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	Url               *string                `protobuf:"bytes,1,opt,name=url" json:"url,omitempty"`
	Mimetype          *string                `protobuf:"bytes,2,opt,name=mimetype" json:"mimetype,omitempty"`
	FileSha256        []byte                 `protobuf:"bytes,3,opt,name=fileSha256" json:"fileSha256,omitempty"`
	FileLength        *uint64                `protobuf:"varint,4,opt,name=fileLength" json:"fileLength,omitempty"`
	MediaKey          []byte                 `protobuf:"bytes,6,opt,name=mediaKey" json:"mediaKey,omitempty"`
	Caption           *string                `protobuf:"bytes,7,opt,name=caption" json:"caption,omitempty"`
	GifPlayback       *bool                  `protobuf:"varint,8,opt,name=gifPlayback" json:"gifPlayback,omitempty"`
	FileEncSha256     []byte                 `protobuf:"bytes,11,opt,name=fileEncSha256" json:"fileEncSha256,omitempty"`
	DirectPath        *string                `protobuf:"bytes,13,opt,name=directPath" json:"directPath,omitempty"`
	MediaKeyTimestamp *int64                 `protobuf:"varint,14,opt,name=mediaKeyTimestamp" json:"mediaKeyTimestamp,omitempty"`
	ContextInfo       *ContextInfo           `protobuf:"bytes,17,opt,name=contextInfo" json:"contextInfo,omitempty"`
	ViewOnce          *bool                  `protobuf:"varint,20,opt,name=viewOnce" json:"viewOnce,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *Message_VideoMessage) Reset() {
	*x = Message_VideoMessage{}
	mi := &file_chatwire_wire_proto_msgTypes[92]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_VideoMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_VideoMessage) ProtoMessage() {}

func (x *Message_VideoMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[92]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_VideoMessage) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 53}
}

func (x *Message_VideoMessage) GetUrl() string {
	if x != nil && x.Url != nil {
		return *x.Url
	}
	return ""
}

func (x *Message_VideoMessage) GetMimetype() string {
	if x != nil && x.Mimetype != nil {
		return *x.Mimetype
	}
	return ""
}

func (x *Message_VideoMessage) GetFileSha256() []byte {
	if x != nil {
		return x.FileSha256
	}
	return nil
}

func (x *Message_VideoMessage) GetFileLength() uint64 {
	if x != nil && x.FileLength != nil {
		return *x.FileLength
	}
	return 0
}

func (x *Message_VideoMessage) GetMediaKey() []byte {
	if x != nil {
		return x.MediaKey
	}
	return nil
}

func (x *Message_VideoMessage) GetCaption() string {
	if x != nil && x.Caption != nil {
		return *x.Caption
	}
	return ""
}

func (x *Message_VideoMessage) GetGifPlayback() bool {
	if x != nil && x.GifPlayback != nil {
		return *x.GifPlayback
	}
	return false
}

func (x *Message_VideoMessage) GetFileEncSha256() []byte {
	if x != nil {
		return x.FileEncSha256
	}
	return nil
}

func (x *Message_VideoMessage) GetDirectPath() string {
	if x != nil && x.DirectPath != nil {
		return *x.DirectPath
	}
	return ""
}

func (x *Message_VideoMessage) GetMediaKeyTimestamp() int64 {
	if x != nil && x.MediaKeyTimestamp != nil {
		return *x.MediaKeyTimestamp
	}
	return 0
}

func (x *Message_VideoMessage) GetContextInfo() *ContextInfo {
	if x != nil {
		return x.ContextInfo
	}
	return nil
}

func (x *Message_VideoMessage) GetViewOnce() bool {
	if x != nil && x.ViewOnce != nil {
		return *x.ViewOnce
	}
	return false
}

type Message_PollCreationMessage_Option struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	OptionName    *string                `protobuf:"bytes,1,opt,name=optionName" json:"optionName,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message_PollCreationMessage_Option) Reset() {
	*x = Message_PollCreationMessage_Option{}
	mi := &file_chatwire_wire_proto_msgTypes[93]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message_PollCreationMessage_Option) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message_PollCreationMessage_Option) ProtoMessage() {}

func (x *Message_PollCreationMessage_Option) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[93]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message_PollCreationMessage_Option) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{14, 37, 0}
}

func (x *Message_PollCreationMessage_Option) GetOptionName() string {
	if x != nil && x.OptionName != nil {
		return *x.OptionName
	}
	return ""
}

type SyncActionValue_ArchiveChatAction struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Archived      *bool                  `protobuf:"varint,1,opt,name=archived" json:"archived,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncActionValue_ArchiveChatAction) Reset() {
	*x = SyncActionValue_ArchiveChatAction{}
	mi := &file_chatwire_wire_proto_msgTypes[94]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncActionValue_ArchiveChatAction) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncActionValue_ArchiveChatAction) ProtoMessage() {}

func (x *SyncActionValue_ArchiveChatAction) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[94]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SyncActionValue_ArchiveChatAction) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{29, 0}
}

func (x *SyncActionValue_ArchiveChatAction) GetArchived() bool {
	if x != nil && x.Archived != nil {
		return *x.Archived
	}
	return false
}

type SyncActionValue_ContactAction struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	FullName      *string                `protobuf:"bytes,1,opt,name=fullName" json:"fullName,omitempty"`
	FirstName     *string                `protobuf:"bytes,2,opt,name=firstName" json:"firstName,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncActionValue_ContactAction) Reset() {
	*x = SyncActionValue_ContactAction{}
	mi := &file_chatwire_wire_proto_msgTypes[95]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncActionValue_ContactAction) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncActionValue_ContactAction) ProtoMessage() {}

func (x *SyncActionValue_ContactAction) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[95]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SyncActionValue_ContactAction) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{29, 1}
}

func (x *SyncActionValue_ContactAction) GetFullName() string {
	if x != nil && x.FullName != nil {
		return *x.FullName
	}
	return ""
}

func (x *SyncActionValue_ContactAction) GetFirstName() string {
	if x != nil && x.FirstName != nil {
		return *x.FirstName
	}
	return ""
}

type SyncActionValue_MuteAction struct {
	state            protoimpl.MessageState `protogen:"open.v1"`
	Muted            *bool                  `protobuf:"varint,1,opt,name=muted" json:"muted,omitempty"`
	MuteEndTimestamp *int64                 `protobuf:"varint,2,opt,name=muteEndTimestamp" json:"muteEndTimestamp,omitempty"`
	unknownFields    protoimpl.UnknownFields
	sizeCache        protoimpl.SizeCache
}

func (x *SyncActionValue_MuteAction) Reset() {
	*x = SyncActionValue_MuteAction{}
	mi := &file_chatwire_wire_proto_msgTypes[96]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncActionValue_MuteAction) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncActionValue_MuteAction) ProtoMessage() {}

func (x *SyncActionValue_MuteAction) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[96]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SyncActionValue_MuteAction) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{29, 2}
}

func (x *SyncActionValue_MuteAction) GetMuted() bool {
	if x != nil && x.Muted != nil {
		return *x.Muted
	}
	return false
}

func (x *SyncActionValue_MuteAction) GetMuteEndTimestamp() int64 {
	if x != nil && x.MuteEndTimestamp != nil {
		return *x.MuteEndTimestamp
	}
	return 0
}

type SyncActionValue_PinAction struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Pinned        *bool                  `protobuf:"varint,1,opt,name=pinned" json:"pinned,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SyncActionValue_PinAction) Reset() {
	*x = SyncActionValue_PinAction{}
	mi := &file_chatwire_wire_proto_msgTypes[97]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SyncActionValue_PinAction) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncActionValue_PinAction) ProtoMessage() {}

func (x *SyncActionValue_PinAction) ProtoReflect() protoreflect.Message {
	mi := &file_chatwire_wire_proto_msgTypes[97]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SyncActionValue_PinAction) Descriptor() ([]byte, []int) {
	return file_chatwire_wire_proto_rawDescGZIP(), []int{29, 3}
}

func (x *SyncActionValue_PinAction) GetPinned() bool {
	if x != nil && x.Pinned != nil {
		return *x.Pinned
	}
	return false
}

var File_chatwire_wire_proto protoreflect.FileDescriptor

const file_chatwire_wire_proto_rawDesc = "" +
	"\n" +
	"\x13chatwire/wire.proto\x12\rchatwire.wire\"\xc0\x01\n" +
	"\x11ADVDeviceIdentity\x12\r\n" +
	"\x05rawId\x18\x01 \x01(\r\x12\x11\n" +
	"\ttimestamp\x18\x02 \x01(\x04\x12\x10\n" +
	"\bkeyIndex\x18\x03 \x01(\r\x12;\n" +
	"\vaccountType\x18\x04 \x01(\x0e2 .chatwire.wire.ADVEncryptionType:\x04E2EE\x12:\n" +
	"\n" +
	"deviceType\x18\x05 \x01(\x0e2 .chatwire.wire.ADVEncryptionType:\x04E2EE\"z\n" +
	"\x17ADVSignedDeviceIdentity\x12\x0f\n" +
	"\adetails\x18\x01 \x01(\f\x12\x1b\n" +
	"\x13accountSignatureKey\x18\x02 \x01(\f\x12\x18\n" +
	"\x10accountSignature\x18\x03 \x01(\f\x12\x17\n" +
	"\x0fdeviceSignature\x18\x04 \x01(\f\"y\n" +
	"\x1bADVSignedDeviceIdentityHMAC\x12\x0f\n" +
	"\adetails\x18\x01 \x01(\f\x12\f\n" +
	"\x04hmac\x18\x02 \x01(\f\x12;\n" +
	"\vaccountType\x18\x03 \x01(\x0e2 .chatwire.wire.ADVEncryptionType:\x04E2EE\"H\n" +
	"\x15AIRichResponseMessage\x12/\n" +
	"\vcontextInfo\x18\x04 \x01(\v2\x1a.chatwire.wire.ContextInfo\"\xc8\x01\n" +
	"\rClientPayload\x12\x10\n" +
	"\busername\x18\x01 \x01(\x04\x12\x0f\n" +
	"\apassive\x18\x03 \x01(\b\x12\x0e\n" +
	"\x06device\x18\x12 \x01(\r\x12U\n" +
	"\x11devicePairingData\x18\x13 \x01(\v2:.chatwire.wire.ClientPayload.DevicePairingRegistrationData\x12\f\n" +
	"\x04pull\x18! \x01(\b\x1a\x1f\n" +
	"\x1dDevicePairingRegistrationData\"\xba\x01\n" +
	"\vContextInfo\x12\x10\n" +
	"\bstanzaId\x18\x01 \x01(\t\x12\x13\n" +
	"\vparticipant\x18\x02 \x01(\t\x12-\n" +
	"\rquotedMessage\x18\x03 \x01(\v2\x16.chatwire.wire.Message\x12\x11\n" +
	"\tremoteJid\x18\x04 \x01(\t\x12\x14\n" +
	"\fmentionedJid\x18\x0f \x03(\t\x12\x17\n" +
	"\x0fforwardingScore\x18\x15 \x01(\r\x12\x13\n" +
	"\visForwarded\x18\x16 \x01(\b\"\x85\x02\n" +
	"\fConversation\x12\n" +
	"\n" +
	"\x02id\x18\x01 \x02(\t\x12/\n" +
	"\bmessages\x18\x02 \x03(\v2\x1d.chatwire.wire.HistorySyncMsg\x12\x18\n" +
	"\x10lastMsgTimestamp\x18\x05 \x01(\x04\x12\x13\n" +
	"\vunreadCount\x18\x06 \x01(\r\x12\x10\n" +
	"\breadOnly\x18\a \x01(\b\x12\x1d\n" +
	"\x15conversationTimestamp\x18\f \x01(\x04\x12\f\n" +
	"\x04name\x18\r \x01(\t\x12\x10\n" +
	"\barchived\x18\x10 \x01(\b\x12\x0e\n" +
	"\x06pinned\x18\x18 \x01(\r\x12\x13\n" +
	"\vmuteEndTime\x18\x19 \x01(\x04\x12\x13\n" +
	"\vdisplayName\x18& \x01(\t\"h\n" +
	"\x15ExternalBlobReference\x12\x10\n" +
	"\bmediaKey\x18\x01 \x01(\f\x12\x12\n" +
	"\n" +
	"directPath\x18\x02 \x01(\t\x12\x12\n" +
	"\n" +
	"fileSha256\x18\x05 \x01(\f\x12\x15\n" +
	"\rfileEncSha256\x18\x06 \x01(\f\"\xde\x03\n" +
	"\vHistorySync\x12<\n" +
	"\bsyncType\x18\x01 \x02(\x0e2*.chatwire.wire.HistorySync.HistorySyncType\x122\n" +
	"\rconversations\x18\x02 \x03(\v2\x1b.chatwire.wire.Conversation\x12\x12\n" +
	"\n" +
	"chunkOrder\x18\x05 \x01(\r\x12\x10\n" +
	"\bprogress\x18\x06 \x01(\r\x12*\n" +
	"\tpushnames\x18\a \x03(\v2\x17.chatwire.wire.Pushname\x12H\n" +
	"\x18phoneNumberToLidMappings\x18\x0f \x03(\v2&.chatwire.wire.PhoneNumberToLIDMapping\x124\n" +
	"\x0einlineContacts\x18\x14 \x03(\v2\x1c.chatwire.wire.InlineContact\"\x8a\x01\n" +
	"\x0fHistorySyncType\x12\x15\n" +
	"\x11INITIAL_BOOTSTRAP\x10\x00\x12\x15\n" +
	"\x11INITIAL_STATUS_V3\x10\x01\x12\b\n" +
	"\x04FULL\x10\x02\x12\n" +
	"\n" +
	"\x06RECENT\x10\x03\x12\r\n" +
	"\tPUSH_NAME\x10\x04\x12\x15\n" +
	"\x11NON_BLOCKING_DATA\x10\x05\x12\r\n" +
	"\tON_DEMAND\x10\x06\"=\n" +
	"\x0eHistorySyncMsg\x12+\n" +
	"\amessage\x18\x01 \x01(\v2\x1a.chatwire.wire.MessageInfo\"S\n" +
	"\rInlineContact\x12\r\n" +
	"\x05pnJid\x18\x01 \x01(\t\x12\x0e\n" +
	"\x06lidJid\x18\x02 \x01(\t\x12\x10\n" +
	"\bfullName\x18\x03 \x01(\t\x12\x11\n" +
	"\tfirstName\x18\x04 \x01(\t\"\x13\n" +
	"\x05KeyId\x12\n" +
	"\n" +
	"\x02id\x18\x01 \x01(\f\"I\n" +
	"\rLegacyMessage\x128\n" +
	"\bpollVote\x18\x02 \x01(\v2&.chatwire.wire.Message.PollVoteMessage\"\xd3\x01\n" +
	"\x16MediaRetryNotification\x12\x10\n" +
	"\bstanzaId\x18\x01 \x01(\t\x12\x12\n" +
	"\n" +
	"directPath\x18\x02 \x01(\t\x12@\n" +
	"\x06result\x18\x03 \x01(\x0e20.chatwire.wire.MediaRetryNotification.ResultType\"Q\n" +
	"\n" +
	"ResultType\x12\x11\n" +
	"\rGENERAL_ERROR\x10\x00\x12\v\n" +
	"\aSUCCESS\x10\x01\x12\r\n" +
	"\tNOT_FOUND\x10\x02\x12\x14\n" +
	"\x10DECRYPTION_ERROR\x10\x03\"\x8ca\n" +
	"\aMessage\x12\x14\n" +
	"\fconversation\x18\x01 \x01(\t\x12Y\n" +
	"\x1csenderKeyDistributionMessage\x18\x02 \x01(\v23.chatwire.wire.Message.SenderKeyDistributionMessage\x129\n" +
	"\fimageMessage\x18\x03 \x01(\v2#.chatwire.wire.Message.ImageMessage\x12=\n" +
	"\x0econtactMessage\x18\x04 \x01(\v2%.chatwire.wire.Message.ContactMessage\x12?\n" +
	"\x0flocationMessage\x18\x05 \x01(\v2&.chatwire.wire.Message.LocationMessage\x12G\n" +
	"\x13extendedTextMessage\x18\x06 \x01(\v2*.chatwire.wire.Message.ExtendedTextMessage\x12?\n" +
	"\x0fdocumentMessage\x18\a \x01(\v2&.chatwire.wire.Message.DocumentMessage\x129\n" +
	"\faudioMessage\x18\b \x01(\v2#.chatwire.wire.Message.AudioMessage\x129\n" +
	"\fvideoMessage\x18\t \x01(\v2#.chatwire.wire.Message.VideoMessage\x12)\n" +
	"\x04call\x18\n" +
	" \x01(\v2\x1b.chatwire.wire.Message.Call\x12?\n" +
	"\x0fprotocolMessage\x18\f \x01(\v2&.chatwire.wire.Message.ProtocolMessage\x12I\n" +
	"\x14contactsArrayMessage\x18\r \x01(\v2+.chatwire.wire.Message.ContactsArrayMessage\x12G\n" +
	"\x13liveLocationMessage\x18\x12 \x01(\v2*.chatwire.wire.Message.LiveLocationMessage\x12?\n" +
	"\x0ftemplateMessage\x18\x19 \x01(\v2&.chatwire.wire.Message.TemplateMessage\x12=\n" +
	"\x0estickerMessage\x18\x1a \x01(\v2%.chatwire.wire.Message.StickerMessage\x12E\n" +
	"\x12groupInviteMessage\x18\x1c \x01(\v2).chatwire.wire.Message.GroupInviteMessage\x12U\n" +
	"\x1atemplateButtonReplyMessage\x18\x1d \x01(\v21.chatwire.wire.Message.TemplateButtonReplyMessage\x12=\n" +
	"\x0eproductMessage\x18\x1e \x01(\v2%.chatwire.wire.Message.ProductMessage\x12C\n" +
	"\x11deviceSentMessage\x18\x1f \x01(\v2(.chatwire.wire.Message.DeviceSentMessage\x12=\n" +
	"\x12messageContextInfo\x18# \x01(\v2!.chatwire.wire.MessageContextInfo\x127\n" +
	"\vlistMessage\x18$ \x01(\v2\".chatwire.wire.Message.ListMessage\x12B\n" +
	"\x0fviewOnceMessage\x18% \x01(\v2).chatwire.wire.Message.FutureProofMessage\x129\n" +
	"\forderMessage\x18& \x01(\v2#.chatwire.wire.Message.OrderMessage\x12G\n" +
	"\x13listResponseMessage\x18' \x01(\v2*.chatwire.wire.Message.ListResponseMessage\x12C\n" +
	"\x10ephemeralMessage\x18( \x01(\v2).chatwire.wire.Message.FutureProofMessage\x12=\n" +
	"\x0ebuttonsMessage\x18* \x01(\v2%.chatwire.wire.Message.ButtonsMessage\x12M\n" +
	"\x16buttonsResponseMessage\x18+ \x01(\v2-.chatwire.wire.Message.ButtonsResponseMessage\x12E\n" +
	"\x12interactiveMessage\x18- \x01(\v2).chatwire.wire.Message.InteractiveMessage\x12?\n" +
	"\x0freactionMessage\x18. \x01(\v2&.chatwire.wire.Message.ReactionMessage\x12U\n" +
	"\x1ainteractiveResponseMessage\x180 \x01(\v21.chatwire.wire.Message.InteractiveResponseMessage\x12G\n" +
	"\x13pollCreationMessage\x181 \x01(\v2*.chatwire.wire.Message.PollCreationMessage\x12C\n" +
	"\x11pollUpdateMessage\x182 \x01(\v2(.chatwire.wire.Message.PollUpdateMessage\x12C\n" +
	"\x11keepInChatMessage\x183 \x01(\v2(.chatwire.wire.Message.KeepInChatMessage\x12M\n" +
	"\x1adocumentWithCaptionMessage\x185 \x01(\v2).chatwire.wire.Message.FutureProofMessage\x12S\n" +
	"\x19requestPhoneNumberMessage\x186 \x01(\v20.chatwire.wire.Message.RequestPhoneNumberMessage\x12D\n" +
	"\x11viewOnceMessageV2\x187 \x01(\v2).chatwire.wire.Message.FutureProofMessage\x12E\n" +
	"\x12encReactionMessage\x188 \x01(\v2).chatwire.wire.Message.EncReactionMessage\x12@\n" +
	"\reditedMessage\x18: \x01(\v2).chatwire.wire.Message.FutureProofMessage\x12M\n" +
	"\x1aviewOnceMessageV2Extension\x18; \x01(\v2).chatwire.wire.Message.FutureProofMessage\x12I\n" +
	"\x15pollCreationMessageV2\x18< \x01(\v2*.chatwire.wire.Message.PollCreationMessage\x12H\n" +
	"\x15groupMentionedMessage\x18> \x01(\v2).chatwire.wire.Message.FutureProofMessage\x12A\n" +
	"\x10pinInChatMessage\x18? \x01(\v2'.chatwire.wire.Message.PinInChatMessage\x12I\n" +
	"\x15pollCreationMessageV3\x18@ \x01(\v2*.chatwire.wire.Message.PollCreationMessage\x127\n" +
	"\n" +
	"ptvMessage\x18B \x01(\v2#.chatwire.wire.Message.VideoMessage\x12C\n" +
	"\x10botInvokeMessage\x18C \x01(\v2).chatwire.wire.Message.FutureProofMessage\x12I\n" +
	"\x14messageHistoryBundle\x18F \x01(\v2+.chatwire.wire.Message.MessageHistoryBundle\x12C\n" +
	"\x11encCommentMessage\x18G \x01(\v2(.chatwire.wire.Message.EncCommentMessage\x12G\n" +
	"\x14lottieStickerMessage\x18J \x01(\v2).chatwire.wire.Message.FutureProofMessage\x129\n" +
	"\feventMessage\x18K \x01(\v2#.chatwire.wire.Message.EventMessage\x12O\n" +
	"\x17encEventResponseMessage\x18L \x01(\v2..chatwire.wire.Message.EncEventResponseMessage\x12Y\n" +
	"\x1cnewsletterAdminInviteMessage\x18N \x01(\v23.chatwire.wire.Message.NewsletterAdminInviteMessage\x12M\n" +
	"\x16secretEncryptedMessage\x18R \x01(\v2-.chatwire.wire.Message.SecretEncryptedMessage\x129\n" +
	"\falbumMessage\x18S \x01(\v2#.chatwire.wire.Message.AlbumMessage\x12E\n" +
	"\x12stickerPackMessage\x18V \x01(\v2).chatwire.wire.Message.StickerPackMessage\x12S\n" +
	"\x19pollResultSnapshotMessage\x18X \x01(\v20.chatwire.wire.Message.PollResultSnapshotMessage\x12Q\n" +
	"\x1epollCreationOptionImageMessage\x18Z \x01(\v2).chatwire.wire.Message.FutureProofMessage\x12I\n" +
	"\x16associatedChildMessage\x18[ \x01(\v2).chatwire.wire.Message.FutureProofMessage\x12A\n" +
	"\x13richResponseMessage\x18a \x01(\v2$.chatwire.wire.AIRichResponseMessage\x12B\n" +
	"\x0fquestionMessage\x18e \x01(\v2).chatwire.wire.Message.FutureProofMessage\x12I\n" +
	"\x14messageHistoryNotice\x18f \x01(\v2+.chatwire.wire.Message.MessageHistoryNotice\x12F\n" +
	"\x13botForwardedMessage\x18h \x01(\v2).chatwire.wire.Message.FutureProofMessage\x12G\n" +
	"\x14questionReplyMessage\x18j \x01(\v2).chatwire.wire.Message.FutureProofMessage\x12I\n" +
	"\x15pollCreationMessageV5\x18o \x01(\v2*.chatwire.wire.Message.PollCreationMessage\x12a\n" +
	"!newsletterFollowerInviteMessageV2\x18q \x01(\v26.chatwire.wire.Message.NewsletterFollowerInviteMessage\x12U\n" +
	"\x1bpollResultSnapshotMessageV3\x18s \x01(\v20.chatwire.wire.Message.PollResultSnapshotMessage\x12A\n" +
	"\x0espoilerMessage\x18v \x01(\v2).chatwire.wire.Message.FutureProofMessage\x12I\n" +
	"\x15pollCreationMessageV6\x18w \x01(\v2*.chatwire.wire.Message.PollCreationMessage\x12E\n" +
	"\x12eventInviteMessage\x18z \x01(\v2).chatwire.wire.Message.EventInviteMessage\x12G\n" +
	"\x13splitPaymentMessage\x18} \x01(\v2*.chatwire.wire.Message.SplitPaymentMessage\x12:\n" +
	"\fmusicMessage\x18\x81\x01 \x01(\v2#.chatwire.wire.Message.MusicMessage\x1a?\n" +
	"\fAlbumMessage\x12/\n" +
	"\vcontextInfo\x18\x11 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a\x87\x01\n" +
	"\x0fAppStateSyncKey\x127\n" +
	"\x05keyId\x18\x01 \x01(\v2(.chatwire.wire.Message.AppStateSyncKeyId\x12;\n" +
	"\akeyData\x18\x02 \x01(\v2*.chatwire.wire.Message.AppStateSyncKeyData\x1a9\n" +
	"\x13AppStateSyncKeyData\x12\x0f\n" +
	"\akeyData\x18\x01 \x01(\f\x12\x11\n" +
	"\ttimestamp\x18\x03 \x01(\x03\x1a\"\n" +
	"\x11AppStateSyncKeyId\x12\r\n" +
	"\x05keyId\x18\x01 \x01(\f\x1aL\n" +
	"\x14AppStateSyncKeyShare\x124\n" +
	"\x04keys\x18\x01 \x03(\v2&.chatwire.wire.Message.AppStateSyncKey\x1a\x8e\x02\n" +
	"\fAudioMessage\x12\v\n" +
	"\x03url\x18\x01 \x01(\t\x12\x10\n" +
	"\bmimetype\x18\x02 \x01(\t\x12\x12\n" +
	"\n" +
	"fileSha256\x18\x03 \x01(\f\x12\x12\n" +
	"\n" +
	"fileLength\x18\x04 \x01(\x04\x12\x0f\n" +
	"\aseconds\x18\x05 \x01(\r\x12\v\n" +
	"\x03ptt\x18\x06 \x01(\b\x12\x10\n" +
	"\bmediaKey\x18\a \x01(\f\x12\x15\n" +
	"\rfileEncSha256\x18\b \x01(\f\x12\x12\n" +
	"\n" +
	"directPath\x18\t \x01(\t\x12\x19\n" +
	"\x11mediaKeyTimestamp\x18\n" +
	" \x01(\x03\x12/\n" +
	"\vcontextInfo\x18\x11 \x01(\v2\x1a.chatwire.wire.ContextInfo\x12\x10\n" +
	"\bviewOnce\x18\x15 \x01(\b\x1aA\n" +
	"\x0eButtonsMessage\x12/\n" +
	"\vcontextInfo\x18\b \x01(\v2\x1a.chatwire.wire.ContextInfo\x1aI\n" +
	"\x16ButtonsResponseMessage\x12/\n" +
	"\vcontextInfo\x18\x03 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a7\n" +
	"\x04Call\x12/\n" +
	"\vcontextInfo\x18\a \x01(\v2\x1a.chatwire.wire.ContextInfo\x1ae\n" +
	"\x0eContactMessage\x12\x13\n" +
	"\vdisplayName\x18\x01 \x01(\t\x12\r\n" +
	"\x05vcard\x18\x10 \x01(\t\x12/\n" +
	"\vcontextInfo\x18\x11 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a\x95\x01\n" +
	"\x14ContactsArrayMessage\x12\x13\n" +
	"\vdisplayName\x18\x01 \x01(\t\x127\n" +
	"\bcontacts\x18\x02 \x03(\v2%.chatwire.wire.Message.ContactMessage\x12/\n" +
	"\vcontextInfo\x18\x11 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1aT\n" +
	"\x11DeviceSentMessage\x12\x16\n" +
	"\x0edestinationJid\x18\x01 \x01(\t\x12'\n" +
	"\amessage\x18\x02 \x01(\v2\x16.chatwire.wire.Message\x1a\x93\x02\n" +
	"\x0fDocumentMessage\x12\v\n" +
	"\x03url\x18\x01 \x01(\t\x12\x10\n" +
	"\bmimetype\x18\x02 \x01(\t\x12\r\n" +
	"\x05title\x18\x03 \x01(\t\x12\x12\n" +
	"\n" +
	"fileSha256\x18\x04 \x01(\f\x12\x12\n" +
	"\n" +
	"fileLength\x18\x05 \x01(\x04\x12\x10\n" +
	"\bmediaKey\x18\a \x01(\f\x12\x10\n" +
	"\bfileName\x18\b \x01(\t\x12\x15\n" +
	"\rfileEncSha256\x18\t \x01(\f\x12\x12\n" +
	"\n" +
	"directPath\x18\n" +
	" \x01(\t\x12\x19\n" +
	"\x11mediaKeyTimestamp\x18\v \x01(\x03\x12/\n" +
	"\vcontextInfo\x18\x11 \x01(\v2\x1a.chatwire.wire.ContextInfo\x12\x0f\n" +
	"\acaption\x18\x14 \x01(\t\x1a\x13\n" +
	"\x11EncCommentMessage\x1a\x19\n" +
	"\x17EncEventResponseMessage\x1a\x14\n" +
	"\x12EncReactionMessage\x1aE\n" +
	"\x12EventInviteMessage\x12/\n" +
	"\vcontextInfo\x18\x01 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a?\n" +
	"\fEventMessage\x12/\n" +
	"\vcontextInfo\x18\x01 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1ai\n" +
	"\x13ExtendedTextMessage\x12\f\n" +
	"\x04text\x18\x01 \x01(\t\x12\x13\n" +
	"\vmatchedText\x18\x02 \x01(\t\x12/\n" +
	"\vcontextInfo\x18\x11 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a=\n" +
	"\x12FutureProofMessage\x12'\n" +
	"\amessage\x18\x01 \x01(\v2\x16.chatwire.wire.Message\x1aE\n" +
	"\x12GroupInviteMessage\x12/\n" +
	"\vcontextInfo\x18\a \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a\xe3\x01\n" +
	"\x17HistorySyncNotification\x12\x12\n" +
	"\n" +
	"fileSha256\x18\x01 \x01(\f\x12\x12\n" +
	"\n" +
	"fileLength\x18\x02 \x01(\x04\x12\x10\n" +
	"\bmediaKey\x18\x03 \x01(\f\x12\x15\n" +
	"\rfileEncSha256\x18\x04 \x01(\f\x12\x12\n" +
	"\n" +
	"directPath\x18\x05 \x01(\t\x128\n" +
	"\bsyncType\x18\x06 \x01(\x0e2&.chatwire.wire.Message.HistorySyncType\x12)\n" +
	"!initialHistBootstrapInlinePayload\x18\v \x01(\f\x1a\xb7\x02\n" +
	"\fImageMessage\x12\v\n" +
	"\x03url\x18\x01 \x01(\t\x12\x10\n" +
	"\bmimetype\x18\x02 \x01(\t\x12\x0f\n" +
	"\acaption\x18\x03 \x01(\t\x12\x12\n" +
	"\n" +
	"fileSha256\x18\x04 \x01(\f\x12\x12\n" +
	"\n" +
	"fileLength\x18\x05 \x01(\x04\x12\x0e\n" +
	"\x06height\x18\x06 \x01(\r\x12\r\n" +
	"\x05width\x18\a \x01(\r\x12\x10\n" +
	"\bmediaKey\x18\b \x01(\f\x12\x15\n" +
	"\rfileEncSha256\x18\t \x01(\f\x12\x12\n" +
	"\n" +
	"directPath\x18\v \x01(\t\x12\x19\n" +
	"\x11mediaKeyTimestamp\x18\f \x01(\x03\x12\x15\n" +
	"\rjpegThumbnail\x18\x10 \x01(\f\x12/\n" +
	"\vcontextInfo\x18\x11 \x01(\v2\x1a.chatwire.wire.ContextInfo\x12\x10\n" +
	"\bviewOnce\x18\x19 \x01(\b\x1aE\n" +
	"\x12InteractiveMessage\x12/\n" +
	"\vcontextInfo\x18\x0f \x01(\v2\x1a.chatwire.wire.ContextInfo\x1aM\n" +
	"\x1aInteractiveResponseMessage\x12/\n" +
	"\vcontextInfo\x18\x0f \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a\x13\n" +
	"\x11KeepInChatMessage\x1a>\n" +
	"\vListMessage\x12/\n" +
	"\vcontextInfo\x18\b \x01(\v2\x1a.chatwire.wire.ContextInfo\x1aF\n" +
	"\x13ListResponseMessage\x12/\n" +
	"\vcontextInfo\x18\x04 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a\x8a\x01\n" +
	"\x13LiveLocationMessage\x12\x17\n" +
	"\x0fdegreesLatitude\x18\x01 \x01(\x01\x12\x18\n" +
	"\x10degreesLongitude\x18\x02 \x01(\x01\x12\x0f\n" +
	"\acaption\x18\x06 \x01(\t\x12/\n" +
	"\vcontextInfo\x18\x11 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a\xc2\x01\n" +
	"\x0fLocationMessage\x12\x17\n" +
	"\x0fdegreesLatitude\x18\x01 \x01(\x01\x12\x18\n" +
	"\x10degreesLongitude\x18\x02 \x01(\x01\x12\f\n" +
	"\x04name\x18\x03 \x01(\t\x12\x0f\n" +
	"\aaddress\x18\x04 \x01(\t\x12\v\n" +
	"\x03url\x18\x05 \x01(\t\x12\x0e\n" +
	"\x06isLive\x18\x06 \x01(\b\x12\x0f\n" +
	"\acomment\x18\v \x01(\t\x12/\n" +
	"\vcontextInfo\x18\x11 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a\xb3\x01\n" +
	"\x14MessageHistoryBundle\x12\x12\n" +
	"\n" +
	"fileSha256\x18\x02 \x01(\f\x12\x10\n" +
	"\bmediaKey\x18\x03 \x01(\f\x12\x15\n" +
	"\rfileEncSha256\x18\x04 \x01(\f\x12\x12\n" +
	"\n" +
	"directPath\x18\x05 \x01(\t\x12\x19\n" +
	"\x11mediaKeyTimestamp\x18\x06 \x01(\x03\x12/\n" +
	"\vcontextInfo\x18\a \x01(\v2\x1a.chatwire.wire.ContextInfo\x1aG\n" +
	"\x14MessageHistoryNotice\x12/\n" +
	"\vcontextInfo\x18\x01 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a?\n" +
	"\fMusicMessage\x12/\n" +
	"\vcontextInfo\x18\x05 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1aO\n" +
	"\x1cNewsletterAdminInviteMessage\x12/\n" +
	"\vcontextInfo\x18\x06 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1aR\n" +
	"\x1fNewsletterFollowerInviteMessage\x12/\n" +
	"\vcontextInfo\x18\x05 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a?\n" +
	"\fOrderMessage\x12/\n" +
	"\vcontextInfo\x18\x11 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a\x12\n" +
	"\x10PinInChatMessage\x1a\xca\x02\n" +
	"\x13PollCreationMessage\x12\f\n" +
	"\x04name\x18\x02 \x01(\t\x12B\n" +
	"\aoptions\x18\x03 \x03(\v21.chatwire.wire.Message.PollCreationMessage.Option\x12\x1e\n" +
	"\x16selectableOptionsCount\x18\x04 \x01(\r\x12/\n" +
	"\vcontextInfo\x18\x05 \x01(\v2\x1a.chatwire.wire.ContextInfo\x12?\n" +
	"\x0fpollContentType\x18\x06 \x01(\x0e2&.chatwire.wire.Message.PollContentType\x121\n" +
	"\bpollType\x18\a \x01(\x0e2\x1f.chatwire.wire.Message.PollType\x1a\x1c\n" +
	"\x06Option\x12\x12\n" +
	"\n" +
	"optionName\x18\x01 \x01(\t\x1a1\n" +
	"\fPollEncValue\x12\x12\n" +
	"\n" +
	"encPayload\x18\x01 \x01(\f\x12\r\n" +
	"\x05encIv\x18\x02 \x01(\f\x1aL\n" +
	"\x19PollResultSnapshotMessage\x12/\n" +
	"\vcontextInfo\x18\x03 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a\x9c\x01\n" +
	"\x11PollUpdateMessage\x129\n" +
	"\x16pollCreationMessageKey\x18\x01 \x01(\v2\x19.chatwire.wire.MessageKey\x121\n" +
	"\x04vote\x18\x02 \x01(\v2#.chatwire.wire.Message.PollEncValue\x12\x19\n" +
	"\x11senderTimestampMs\x18\x04 \x01(\x03\x1a*\n" +
	"\x0fPollVoteMessage\x12\x17\n" +
	"\x0fselectedOptions\x18\x01 \x03(\f\x1aA\n" +
	"\x0eProductMessage\x12/\n" +
	"\vcontextInfo\x18\x11 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a\xea\n" +
	"\n" +
	"\x0fProtocolMessage\x12&\n" +
	"\x03key\x18\x01 \x01(\v2\x19.chatwire.wire.MessageKey\x129\n" +
	"\x04type\x18\x02 \x01(\x0e2+.chatwire.wire.Message.ProtocolMessage.Type\x12O\n" +
	"\x17historySyncNotification\x18\x06 \x01(\v2..chatwire.wire.Message.HistorySyncNotification\x12I\n" +
	"\x14appStateSyncKeyShare\x18\a \x01(\v2+.chatwire.wire.Message.AppStateSyncKeyShare\x12-\n" +
	"\reditedMessage\x18\x0e \x01(\v2\x16.chatwire.wire.Message\x12\x13\n" +
	"\vtimestampMs\x18\x0f \x01(\x03\"\x93\b\n" +
	"\x04Type\x12\n" +
	"\n" +
	"\x06REVOKE\x10\x00\x12\x15\n" +
	"\x11EPHEMERAL_SETTING\x10\x03\x12\x1b\n" +
	"\x17EPHEMERAL_SYNC_RESPONSE\x10\x04\x12\x1d\n" +
	"\x19HISTORY_SYNC_NOTIFICATION\x10\x05\x12\x1c\n" +
	"\x18APP_STATE_SYNC_KEY_SHARE\x10\x06\x12\x1e\n" +
	"\x1aAPP_STATE_SYNC_KEY_REQUEST\x10\a\x12\x1f\n" +
	"\x1bMSG_FANOUT_BACKFILL_REQUEST\x10\b\x12.\n" +
	"*INITIAL_SECURITY_NOTIFICATION_SETTING_SYNC\x10\t\x12*\n" +
	"&APP_STATE_FATAL_EXCEPTION_NOTIFICATION\x10\n" +
	"\x12\x16\n" +
	"\x12SHARE_PHONE_NUMBER\x10\v\x12\x10\n" +
	"\fMESSAGE_EDIT\x10\x0e\x12'\n" +
	"#PEER_DATA_OPERATION_REQUEST_MESSAGE\x10\x10\x120\n" +
	",PEER_DATA_OPERATION_REQUEST_RESPONSE_MESSAGE\x10\x11\x12\x1b\n" +
	"\x17REQUEST_WELCOME_MESSAGE\x10\x12\x12\x18\n" +
	"\x14BOT_FEEDBACK_MESSAGE\x10\x13\x12\x18\n" +
	"\x14MEDIA_NOTIFY_MESSAGE\x10\x14\x12)\n" +
	"%CLOUD_API_THREAD_CONTROL_NOTIFICATION\x10\x15\x12\x1e\n" +
	"\x1aLID_MIGRATION_MAPPING_SYNC\x10\x16\x12\x14\n" +
	"\x10REMINDER_MESSAGE\x10\x17\x12\x1f\n" +
	"\x1bBOT_MEMU_ONBOARDING_MESSAGE\x10\x18\x12\x1a\n" +
	"\x16STATUS_MENTION_MESSAGE\x10\x19\x12\x1b\n" +
	"\x17STOP_GENERATION_MESSAGE\x10\x1a\x12\x11\n" +
	"\rLIMIT_SHARING\x10\x1b\x12\x13\n" +
	"\x0fAI_PSI_METADATA\x10\x1c\x12\x13\n" +
	"\x0fAI_QUERY_FANOUT\x10\x1d\x12\x1d\n" +
	"\x19GROUP_MEMBER_LABEL_CHANGE\x10\x1e\x12\x1f\n" +
	"\x1bAI_MEDIA_COLLECTION_MESSAGE\x10\x1f\x12\x16\n" +
	"\x12MESSAGE_UNSCHEDULE\x10 \x12\x16\n" +
	"\x12CHAT_THEME_SETTING\x10\"\x12\x19\n" +
	"\x15AI_METADATA_OPERATION\x10#\x12\x1b\n" +
	"\x17MARK_AS_VERIFIED_ACTION\x10$\x12\x13\n" +
	"\x0fCOEX_STATE_SYNC\x10%\x12\x10\n" +
	"\fACP2_SETTING\x10'\x12(\n" +
	"$SHARED_DEVICE_CONTACT_HASH_KEY_SHARE\x10(\x12*\n" +
	"&SHARED_DEVICE_CONTACT_HASH_KEY_REQUEST\x10)\x1ab\n" +
	"\x0fReactionMessage\x12&\n" +
	"\x03key\x18\x01 \x01(\v2\x19.chatwire.wire.MessageKey\x12\f\n" +
	"\x04text\x18\x02 \x01(\t\x12\x19\n" +
	"\x11senderTimestampMs\x18\x04 \x01(\x03\x1aL\n" +
	"\x19RequestPhoneNumberMessage\x12/\n" +
	"\vcontextInfo\x18\x01 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a\xe6\x01\n" +
	"\x16SecretEncryptedMessage\x12R\n" +
	"\rsecretEncType\x18\x04 \x01(\x0e2;.chatwire.wire.Message.SecretEncryptedMessage.SecretEncType\"x\n" +
	"\rSecretEncType\x12\v\n" +
	"\aUNKNOWN\x10\x00\x12\x0e\n" +
	"\n" +
	"EVENT_EDIT\x10\x01\x12\x10\n" +
	"\fMESSAGE_EDIT\x10\x02\x12\x14\n" +
	"\x10MESSAGE_SCHEDULE\x10\x03\x12\r\n" +
	"\tPOLL_EDIT\x10\x04\x12\x13\n" +
	"\x0fPOLL_ADD_OPTION\x10\x05\x1a\\\n" +
	"\x1cSenderKeyDistributionMessage\x12\x0f\n" +
	"\agroupId\x18\x01 \x01(\t\x12+\n" +
	"#axolotlSenderKeyDistributionMessage\x18\x02 \x01(\f\x1aF\n" +
	"\x13SplitPaymentMessage\x12/\n" +
	"\vcontextInfo\x18\x11 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a\xe0\x01\n" +
	"\x0eStickerMessage\x12\v\n" +
	"\x03url\x18\x01 \x01(\t\x12\x12\n" +
	"\n" +
	"fileSha256\x18\x02 \x01(\f\x12\x15\n" +
	"\rfileEncSha256\x18\x03 \x01(\f\x12\x10\n" +
	"\bmediaKey\x18\x04 \x01(\f\x12\x10\n" +
	"\bmimetype\x18\x05 \x01(\t\x12\x12\n" +
	"\n" +
	"directPath\x18\b \x01(\t\x12\x12\n" +
	"\n" +
	"fileLength\x18\t \x01(\x04\x12\x19\n" +
	"\x11mediaKeyTimestamp\x18\n" +
	" \x01(\x03\x12/\n" +
	"\vcontextInfo\x18\x11 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a\xc5\x01\n" +
	"\x12StickerPackMessage\x12\x12\n" +
	"\n" +
	"fileLength\x18\x05 \x01(\x04\x12\x12\n" +
	"\n" +
	"fileSha256\x18\x06 \x01(\f\x12\x15\n" +
	"\rfileEncSha256\x18\a \x01(\f\x12\x10\n" +
	"\bmediaKey\x18\b \x01(\f\x12\x12\n" +
	"\n" +
	"directPath\x18\t \x01(\t\x12/\n" +
	"\vcontextInfo\x18\v \x01(\v2\x1a.chatwire.wire.ContextInfo\x12\x19\n" +
	"\x11mediaKeyTimestamp\x18\r \x01(\x03\x1aM\n" +
	"\x1aTemplateButtonReplyMessage\x12/\n" +
	"\vcontextInfo\x18\x03 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1aB\n" +
	"\x0fTemplateMessage\x12/\n" +
	"\vcontextInfo\x18\x03 \x01(\v2\x1a.chatwire.wire.ContextInfo\x1a\x96\x02\n" +
	"\fVideoMessage\x12\v\n" +
	"\x03url\x18\x01 \x01(\t\x12\x10\n" +
	"\bmimetype\x18\x02 \x01(\t\x12\x12\n" +
	"\n" +
	"fileSha256\x18\x03 \x01(\f\x12\x12\n" +
	"\n" +
	"fileLength\x18\x04 \x01(\x04\x12\x10\n" +
	"\bmediaKey\x18\x06 \x01(\f\x12\x0f\n" +
	"\acaption\x18\a \x01(\t\x12\x13\n" +
	"\vgifPlayback\x18\b \x01(\b\x12\x15\n" +
	"\rfileEncSha256\x18\v \x01(\f\x12\x12\n" +
	"\n" +
	"directPath\x18\r \x01(\t\x12\x19\n" +
	"\x11mediaKeyTimestamp\x18\x0e \x01(\x03\x12/\n" +
	"\vcontextInfo\x18\x11 \x01(\v2\x1a.chatwire.wire.ContextInfo\x12\x10\n" +
	"\bviewOnce\x18\x14 \x01(\b\"\xb5\x01\n" +
	"\x0fHistorySyncType\x12\x15\n" +
	"\x11INITIAL_BOOTSTRAP\x10\x00\x12\x15\n" +
	"\x11INITIAL_STATUS_V3\x10\x01\x12\b\n" +
	"\x04FULL\x10\x02\x12\n" +
	"\n" +
	"\x06RECENT\x10\x03\x12\r\n" +
	"\tPUSH_NAME\x10\x04\x12\x15\n" +
	"\x11NON_BLOCKING_DATA\x10\x05\x12\r\n" +
	"\tON_DEMAND\x10\x06\x12\x0e\n" +
	"\n" +
	"NO_HISTORY\x10\a\x12\x19\n" +
	"\x15MESSAGE_ACCESS_STATUS\x10\b\"3\n" +
	"\x0fPollContentType\x12\v\n" +
	"\aUNKNOWN\x10\x00\x12\b\n" +
	"\x04TEXT\x10\x01\x12\t\n" +
	"\x05IMAGE\x10\x02\"\x1e\n" +
	"\bPollType\x12\b\n" +
	"\x04POLL\x10\x00\x12\b\n" +
	"\x04QUIZ\x10\x01\"\xc1\x02\n" +
	"\fMessageAddOn\x12F\n" +
	"\x10messageAddOnType\x18\x01 \x01(\x0e2,.chatwire.wire.MessageAddOn.MessageAddOnType\x12\x19\n" +
	"\x11senderTimestampMs\x18\x03 \x01(\x03\x122\n" +
	"\x0fmessageAddOnKey\x18\a \x01(\v2\x19.chatwire.wire.MessageKey\x123\n" +
	"\rlegacyMessage\x18\b \x01(\v2\x1c.chatwire.wire.LegacyMessage\"e\n" +
	"\x10MessageAddOnType\x12\r\n" +
	"\tUNDEFINED\x10\x00\x12\f\n" +
	"\bREACTION\x10\x01\x12\x12\n" +
	"\x0eEVENT_RESPONSE\x10\x02\x12\x0f\n" +
	"\vPOLL_UPDATE\x10\x03\x12\x0f\n" +
	"\vPIN_IN_CHAT\x10\x04\"+\n" +
	"\x12MessageContextInfo\x12\x15\n" +
	"\rmessageSecret\x18\x03 \x01(\f\"\xc6\x02\n" +
	"\vMessageInfo\x12&\n" +
	"\x03key\x18\x01 \x02(\v2\x19.chatwire.wire.MessageKey\x12'\n" +
	"\amessage\x18\x02 \x01(\v2\x16.chatwire.wire.Message\x12\x18\n" +
	"\x10messageTimestamp\x18\x03 \x01(\x04\x12\x13\n" +
	"\vparticipant\x18\x05 \x01(\t\x12\x10\n" +
	"\bpushName\x18\x13 \x01(\t\x12*\n" +
	"\treactions\x18) \x03(\v2\x17.chatwire.wire.Reaction\x12.\n" +
	"\vpollUpdates\x18- \x03(\v2\x19.chatwire.wire.PollUpdate\x12\x15\n" +
	"\rmessageSecret\x181 \x01(\f\x122\n" +
	"\rmessageAddOns\x18D \x03(\v2\x1b.chatwire.wire.MessageAddOn\"P\n" +
	"\n" +
	"MessageKey\x12\x11\n" +
	"\tremoteJid\x18\x01 \x01(\t\x12\x0e\n" +
	"\x06fromMe\x18\x02 \x01(\b\x12\n" +
	"\n" +
	"\x02id\x18\x03 \x01(\t\x12\x13\n" +
	"\vparticipant\x18\x04 \x01(\t\"8\n" +
	"\x17PhoneNumberToLIDMapping\x12\r\n" +
	"\x05pnJid\x18\x01 \x01(\t\x12\x0e\n" +
	"\x06lidJid\x18\x02 \x01(\t\"\x96\x01\n" +
	"\n" +
	"PollUpdate\x127\n" +
	"\x14pollUpdateMessageKey\x18\x01 \x01(\v2\x19.chatwire.wire.MessageKey\x124\n" +
	"\x04vote\x18\x02 \x01(\v2&.chatwire.wire.Message.PollVoteMessage\x12\x19\n" +
	"\x11senderTimestampMs\x18\x03 \x01(\x03\"\x8e\x01\n" +
	"\x13PreKeySignalMessage\x12\x10\n" +
	"\bpreKeyId\x18\x01 \x01(\r\x12\x0f\n" +
	"\abaseKey\x18\x02 \x01(\f\x12\x13\n" +
	"\videntityKey\x18\x03 \x01(\f\x12\x0f\n" +
	"\amessage\x18\x04 \x01(\f\x12\x16\n" +
	"\x0eregistrationId\x18\x05 \x01(\r\x12\x16\n" +
	"\x0esignedPreKeyId\x18\x06 \x01(\r\"(\n" +
	"\bPushname\x12\n" +
	"\n" +
	"\x02id\x18\x01 \x01(\t\x12\x10\n" +
	"\bpushname\x18\x02 \x01(\t\"[\n" +
	"\bReaction\x12&\n" +
	"\x03key\x18\x01 \x01(\v2\x19.chatwire.wire.MessageKey\x12\f\n" +
	"\x04text\x18\x02 \x01(\t\x12\x19\n" +
	"\x11senderTimestampMs\x18\x04 \x01(\x03\"c\n" +
	"\x1cSenderKeyDistributionMessage\x12\n" +
	"\n" +
	"\x02id\x18\x01 \x01(\r\x12\x11\n" +
	"\titeration\x18\x02 \x01(\r\x12\x10\n" +
	"\bchainKey\x18\x03 \x01(\f\x12\x12\n" +
	"\n" +
	"signingKey\x18\x04 \x01(\f\"E\n" +
	"\x10SenderKeyMessage\x12\n" +
	"\n" +
	"\x02id\x18\x01 \x01(\r\x12\x11\n" +
	"\titeration\x18\x02 \x01(\r\x12\x12\n" +
	"\n" +
	"ciphertext\x18\x03 \x01(\f\"&\n" +
	"\x12ServerErrorReceipt\x12\x10\n" +
	"\bstanzaId\x18\x01 \x01(\t\"a\n" +
	"\rSignalMessage\x12\x12\n" +
	"\n" +
	"ratchetKey\x18\x01 \x01(\f\x12\x0f\n" +
	"\acounter\x18\x02 \x01(\r\x12\x17\n" +
	"\x0fpreviousCounter\x18\x03 \x01(\r\x12\x12\n" +
	"\n" +
	"ciphertext\x18\x04 \x01(\f\"p\n" +
	"\x0eSyncActionData\x12\r\n" +
	"\x05index\x18\x01 \x01(\f\x12-\n" +
	"\x05value\x18\x02 \x01(\v2\x1e.chatwire.wire.SyncActionValue\x12\x0f\n" +
	"\apadding\x18\x03 \x01(\f\x12\x0f\n" +
	"\aversion\x18\x04 \x01(\x05\"\xe3\x03\n" +
	"\x0fSyncActionValue\x12\x11\n" +
	"\ttimestamp\x18\x01 \x01(\x03\x12C\n" +
	"\rcontactAction\x18\x03 \x01(\v2,.chatwire.wire.SyncActionValue.ContactAction\x12=\n" +
	"\n" +
	"muteAction\x18\x04 \x01(\v2).chatwire.wire.SyncActionValue.MuteAction\x12;\n" +
	"\tpinAction\x18\x05 \x01(\v2(.chatwire.wire.SyncActionValue.PinAction\x12K\n" +
	"\x11archiveChatAction\x18\x11 \x01(\v20.chatwire.wire.SyncActionValue.ArchiveChatAction\x1a%\n" +
	"\x11ArchiveChatAction\x12\x10\n" +
	"\barchived\x18\x01 \x01(\b\x1a4\n" +
	"\rContactAction\x12\x10\n" +
	"\bfullName\x18\x01 \x01(\t\x12\x11\n" +
	"\tfirstName\x18\x02 \x01(\t\x1a5\n" +
	"\n" +
	"MuteAction\x12\r\n" +
	"\x05muted\x18\x01 \x01(\b\x12\x18\n" +
	"\x10muteEndTimestamp\x18\x02 \x01(\x03\x1a\x1b\n" +
	"\tPinAction\x12\x0e\n" +
	"\x06pinned\x18\x01 \x01(\b\"\x1a\n" +
	"\n" +
	"SyncdIndex\x12\f\n" +
	"\x04blob\x18\x01 \x01(\f\"\xa2\x01\n" +
	"\rSyncdMutation\x12>\n" +
	"\toperation\x18\x01 \x01(\x0e2+.chatwire.wire.SyncdMutation.SyncdOperation\x12*\n" +
	"\x06record\x18\x02 \x01(\v2\x1a.chatwire.wire.SyncdRecord\"%\n" +
	"\x0eSyncdOperation\x12\a\n" +
	"\x03SET\x10\x00\x12\n" +
	"\n" +
	"\x06REMOVE\x10\x01\"A\n" +
	"\x0eSyncdMutations\x12/\n" +
	"\tmutations\x18\x01 \x03(\v2\x1c.chatwire.wire.SyncdMutation\"\xf8\x01\n" +
	"\n" +
	"SyncdPatch\x12,\n" +
	"\aversion\x18\x01 \x01(\v2\x1b.chatwire.wire.SyncdVersion\x12/\n" +
	"\tmutations\x18\x02 \x03(\v2\x1c.chatwire.wire.SyncdMutation\x12?\n" +
	"\x11externalMutations\x18\x03 \x01(\v2$.chatwire.wire.ExternalBlobReference\x12\x13\n" +
	"\vsnapshotMac\x18\x04 \x01(\f\x12\x10\n" +
	"\bpatchMac\x18\x05 \x01(\f\x12#\n" +
	"\x05keyId\x18\x06 \x01(\v2\x14.chatwire.wire.KeyId\"\x86\x01\n" +
	"\vSyncdRecord\x12(\n" +
	"\x05index\x18\x01 \x01(\v2\x19.chatwire.wire.SyncdIndex\x12(\n" +
	"\x05value\x18\x02 \x01(\v2\x19.chatwire.wire.SyncdValue\x12#\n" +
	"\x05keyId\x18\x03 \x01(\v2\x14.chatwire.wire.KeyId\"\x9c\x01\n" +
	"\rSyncdSnapshot\x12,\n" +
	"\aversion\x18\x01 \x01(\v2\x1b.chatwire.wire.SyncdVersion\x12+\n" +
	"\arecords\x18\x02 \x03(\v2\x1a.chatwire.wire.SyncdRecord\x12\v\n" +
	"\x03mac\x18\x03 \x01(\f\x12#\n" +
	"\x05keyId\x18\x04 \x01(\v2\x14.chatwire.wire.KeyId\"\x1a\n" +
	"\n" +
	"SyncdValue\x12\f\n" +
	"\x04blob\x18\x01 \x01(\f\"\x1f\n" +
	"\fSyncdVersion\x12\x0f\n" +
	"\aversion\x18\x01 \x01(\x04*7\n" +
	"\x11ADVEncryptionType\x12\b\n" +
	"\x04E2EE\x10\x00\x12\n" +
	"\n" +
	"\x06HOSTED\x10\x01\x12\f\n" +
	"\bNON_E2EE\x10\x02B\x1dZ\x1bchatwire/internal/wire;wireb\x06proto2"

var (
	file_chatwire_wire_proto_rawDescOnce sync.Once
	file_chatwire_wire_proto_rawDescData []byte
)

func file_chatwire_wire_proto_rawDescGZIP() []byte {
	file_chatwire_wire_proto_rawDescOnce.Do(func() {
		file_chatwire_wire_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_chatwire_wire_proto_rawDesc), len(file_chatwire_wire_proto_rawDesc)))
	})
	return file_chatwire_wire_proto_rawDescData
}

var file_chatwire_wire_proto_enumTypes = make([]protoimpl.EnumInfo, 10)
var file_chatwire_wire_proto_msgTypes = make([]protoimpl.MessageInfo, 98)
var file_chatwire_wire_proto_goTypes = []any{
	(ADVEncryptionType)(0),
	(HistorySync_HistorySyncType)(0),
	(MediaRetryNotification_ResultType)(0),
	(Message_HistorySyncType)(0),
	(Message_PollContentType)(0),
	(Message_PollType)(0),
	(Message_ProtocolMessage_Type)(0),
	(Message_SecretEncryptedMessage_SecretEncType)(0),
	(MessageAddOn_MessageAddOnType)(0),
	(SyncdMutation_SyncdOperation)(0),
	(*ADVDeviceIdentity)(nil),
	(*ADVSignedDeviceIdentity)(nil),
	(*ADVSignedDeviceIdentityHMAC)(nil),
	(*AIRichResponseMessage)(nil),
	(*ClientPayload)(nil),
	(*ContextInfo)(nil),
	(*Conversation)(nil),
	(*ExternalBlobReference)(nil),
	(*HistorySync)(nil),
	(*HistorySyncMsg)(nil),
	(*InlineContact)(nil),
	(*KeyId)(nil),
	(*LegacyMessage)(nil),
	(*MediaRetryNotification)(nil),
	(*Message)(nil),
	(*MessageAddOn)(nil),
	(*MessageContextInfo)(nil),
	(*MessageInfo)(nil),
	(*MessageKey)(nil),
	(*PhoneNumberToLIDMapping)(nil),
	(*PollUpdate)(nil),
	(*PreKeySignalMessage)(nil),
	(*Pushname)(nil),
	(*Reaction)(nil),
	(*SenderKeyDistributionMessage)(nil),
	(*SenderKeyMessage)(nil),
	(*ServerErrorReceipt)(nil),
	(*SignalMessage)(nil),
	(*SyncActionData)(nil),
	(*SyncActionValue)(nil),
	(*SyncdIndex)(nil),
	(*SyncdMutation)(nil),
	(*SyncdMutations)(nil),
	(*SyncdPatch)(nil),
	(*SyncdRecord)(nil),
	(*SyncdSnapshot)(nil),
	(*SyncdValue)(nil),
	(*SyncdVersion)(nil),
	(*ClientPayload_DevicePairingRegistrationData)(nil),
	(*Message_AlbumMessage)(nil),
	(*Message_AppStateSyncKey)(nil),
	(*Message_AppStateSyncKeyData)(nil),
	(*Message_AppStateSyncKeyId)(nil),
	(*Message_AppStateSyncKeyShare)(nil),
	(*Message_AudioMessage)(nil),
	(*Message_ButtonsMessage)(nil),
	(*Message_ButtonsResponseMessage)(nil),
	(*Message_Call)(nil),
	(*Message_ContactMessage)(nil),
	(*Message_ContactsArrayMessage)(nil),
	(*Message_DeviceSentMessage)(nil),
	(*Message_DocumentMessage)(nil),
	(*Message_EncCommentMessage)(nil),
	(*Message_EncEventResponseMessage)(nil),
	(*Message_EncReactionMessage)(nil),
	(*Message_EventInviteMessage)(nil),
	(*Message_EventMessage)(nil),
	(*Message_ExtendedTextMessage)(nil),
	(*Message_FutureProofMessage)(nil),
	(*Message_GroupInviteMessage)(nil),
	(*Message_HistorySyncNotification)(nil),
	(*Message_ImageMessage)(nil),
	(*Message_InteractiveMessage)(nil),
	(*Message_InteractiveResponseMessage)(nil),
	(*Message_KeepInChatMessage)(nil),
	(*Message_ListMessage)(nil),
	(*Message_ListResponseMessage)(nil),
	(*Message_LiveLocationMessage)(nil),
	(*Message_LocationMessage)(nil),
	(*Message_MessageHistoryBundle)(nil),
	(*Message_MessageHistoryNotice)(nil),
	(*Message_MusicMessage)(nil),
	(*Message_NewsletterAdminInviteMessage)(nil),
	(*Message_NewsletterFollowerInviteMessage)(nil),
	(*Message_OrderMessage)(nil),
	(*Message_PinInChatMessage)(nil),
	(*Message_PollCreationMessage)(nil),
	(*Message_PollEncValue)(nil),
	(*Message_PollResultSnapshotMessage)(nil),
	(*Message_PollUpdateMessage)(nil),
	(*Message_PollVoteMessage)(nil),
	(*Message_ProductMessage)(nil),
	(*Message_ProtocolMessage)(nil),
	(*Message_ReactionMessage)(nil),
	(*Message_RequestPhoneNumberMessage)(nil),
	(*Message_SecretEncryptedMessage)(nil),
	(*Message_SenderKeyDistributionMessage)(nil),
	(*Message_SplitPaymentMessage)(nil),
	(*Message_StickerMessage)(nil),
	(*Message_StickerPackMessage)(nil),
	(*Message_TemplateButtonReplyMessage)(nil),
	(*Message_TemplateMessage)(nil),
	(*Message_VideoMessage)(nil),
	(*Message_PollCreationMessage_Option)(nil),
	(*SyncActionValue_ArchiveChatAction)(nil),
	(*SyncActionValue_ContactAction)(nil),
	(*SyncActionValue_MuteAction)(nil),
	(*SyncActionValue_PinAction)(nil),
}
var file_chatwire_wire_proto_depIdxs = []int32{
	0,
	0,
	0,
	15,
	48,
	24,
	19,
	1,
	16,
	32,
	29,
	20,
	27,
	90,
	2,
	96,
	71,
	58,
	78,
	67,
	61,
	54,
	102,
	57,
	92,
	59,
	77,
	101,
	98,
	69,
	100,
	91,
	60,
	26,
	75,
	68,
	84,
	76,
	68,
	55,
	56,
	72,
	93,
	73,
	86,
	89,
	74,
	68,
	94,
	68,
	64,
	68,
	68,
	86,
	68,
	85,
	86,
	102,
	68,
	79,
	62,
	68,
	66,
	63,
	82,
	95,
	49,
	99,
	88,
	68,
	68,
	13,
	68,
	80,
	68,
	68,
	86,
	83,
	88,
	68,
	86,
	65,
	97,
	81,
	8,
	28,
	22,
	28,
	24,
	33,
	30,
	25,
	28,
	90,
	28,
	39,
	105,
	106,
	107,
	104,
	9,
	44,
	41,
	47,
	41,
	17,
	21,
	40,
	46,
	21,
	47,
	44,
	21,
	15,
	52,
	51,
	50,
	15,
	15,
	15,
	15,
	15,
	58,
	15,
	24,
	15,
	15,
	15,
	15,
	24,
	15,
	3,
	15,
	15,
	15,
	15,
	15,
	15,
	15,
	15,
	15,
	15,
	15,
	15,
	15,
	103,
	15,
	4,
	5,
	15,
	28,
	87,
	15,
	28,
	6,
	70,
	53,
	24,
	28,
	15,
	7,
	15,
	15,
	15,
	15,
	15,
	15,
	167,
	167,
	167,
	167,
	0,
}

func init() { file_chatwire_wire_proto_init() }
func file_chatwire_wire_proto_init() {
	if File_chatwire_wire_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_chatwire_wire_proto_rawDesc), len(file_chatwire_wire_proto_rawDesc)),
			NumEnums:      10,
			NumMessages:   98,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_chatwire_wire_proto_goTypes,
		DependencyIndexes: file_chatwire_wire_proto_depIdxs,
		EnumInfos:         file_chatwire_wire_proto_enumTypes,
		MessageInfos:      file_chatwire_wire_proto_msgTypes,
	}.Build()
	File_chatwire_wire_proto = out.File
	file_chatwire_wire_proto_goTypes = nil
	file_chatwire_wire_proto_depIdxs = nil
}
