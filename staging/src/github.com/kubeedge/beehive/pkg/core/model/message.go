package model

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// Constants for database operations and resource type settings
const (
	InsertOperation        = "insert"
	DeleteOperation        = "delete"
	QueryOperation         = "query"
	UpdateOperation        = "update"
	ResponseOperation      = "response"
	ResponseErrorOperation = "error"

	ResourceTypePod        = "pod"
	ResourceTypeConfigmap  = "configmap"
	ResourceTypeSecret     = "secret"
	ResourceTypeNode       = "node"
	ResourceTypePodlist    = "podlist"
	ResourceTypePodStatus  = "podstatus"
	ResourceTypeNodeStatus = "nodestatus"
)

// Message struct
type Message struct {
	Header  MessageHeader `json:"header"`
	Router  MessageRoute  `json:"route,omitempty"`
	// Meta save the meta info of message content
	ContentMeta ResourceMeta `json:"ResourceMeta,omitempty"`
	Content interface{}   `json:"content"`
}

// MessageRoute contains structure of message
type MessageRoute struct {
	// where the message come from
	Source string `json:"source,omitempty"`
	// which module/group the message will send to
	Dest string `json:"source,omitempty"`
	// is the message unicast or broadcast
	IsBroadcast bool `json:"IsBroadcast,omitempty"`
}

// MessageHeader defines message header details
type MessageHeader struct {
	// the message uuid
	ID string `json:"msg_id"`
	// parentID  means messages is a response message or not
	// please use NewRespByMessage to new response message
	ParentID string `json:"parent_msg_id,omitempty"`
	// the time of creating
	Timestamp int64 `json:"timestamp"`
	// Is the message sync or async
	Sync bool `json:"sync,omitempty"`
}

type ResourceMeta struct{
	Namespace string `json:"namespace,omitempty"`
	Type string `json:"type,omitempty"`
	Name string `json:"name,omitempty"`
	Operation string `json:"operation,omitempty"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

// SetHeader builds message header. You can also use for updating message header
func (msg *Message) SetHeaderRaw(ID, parentID string, timestamp int64) *Message {
	msg.Header.ID = ID
	msg.Header.ParentID = parentID
	msg.Header.Timestamp = timestamp
	return msg
}
func (msg *Message) SetHeader(header MessageHeader) *Message {
	msg.Header = header
	return msg
}
// SetRouteRaw sets router info in message
func (msg *Message) SetRouterRaw(source, dest string, isBroadcast bool) *Message {
	msg.Router.Source = source
	msg.Router.Dest = dest
	msg.Router.IsBroadcast = isBroadcast
	return msg
}
func (msg *Message) SetRouter(router MessageRoute) *Message {
	msg.Router = router
	return msg
}

// SetContentMetaRaw sets content meta info in message
func (msg *Message) SetContentMetaRaw(ns,ty,name,opr,resv  string) *Message{
	msg.ContentMeta.Namespace = ns
	msg.ContentMeta.Type = ty
	msg.ContentMeta.Name = name
	msg.ContentMeta.Operation = opr
	msg.ContentMeta.ResourceVersion = resv
	return msg
}

func (msg *Message) SetContentMeta(meta ResourceMeta) *Message {
	msg.ContentMeta = meta
	return msg
}

//FillBody fills message  content that you want to send
func (msg *Message) FillBody(content interface{}) *Message {
	msg.Content = content
	return msg
}

//GetContent returns header
func (msg *Message) GetHeader() MessageHeader{
	return msg.Header
}

//GetContent returns router
func (msg *Message) GetRouter() MessageRoute{
	return msg.Router
}

//GetContent returns contentMeta
func (msg *Message) GetContentMeta() ResourceMeta{
	return msg.ContentMeta
}

//GetContent returns message content
func (msg *Message) GetContent() interface{} {
	return msg.Content
}

// SetResourceVersion sets resource version in message header
func (msg *Message) SetResourceVersion(resourceVersion string) *Message {
	msg.ContentMeta.ResourceVersion = resourceVersion
	return msg
}

// Get Header
// GetID returns message ID
func (msg *Message) GetID() string {
	return msg.Header.ID
}

//GetParentID returns message parent id
func (msg *Message) GetParentID() string {
	return msg.Header.ParentID
}

//GetTimestamp returns message timestamp
func (msg *Message) GetTimestamp() int64 {
	return msg.Header.Timestamp
}
// IsSync : show the msg is sync or async
func (msg *Message) IsSync() bool {
	return msg.Header.Sync
}

// Get Router
// GetSource returns message route source string
func (msg *Message) GetSource() string {
	return msg.Router.Source
}
// GetDest returns message route group
func (msg *Message) GetDest() string {
	return msg.Router.Dest
}

func (msg *Message) IsBroadcast()bool{
	return msg.Router.IsBroadcast
}

// Get ContentMeta
func (msg *Message) GetContentNS() string {
	return msg.ContentMeta.Namespace
}

func (msg *Message) GetContentType() string {
	return msg.ContentMeta.Type
}
func (msg *Message) GetContentName() string {
	return msg.ContentMeta.Name
}
func (msg *Message) GetResourceVersion() string {
	return msg.ContentMeta.ResourceVersion
}
func (msg *Message) GetOperation() string {
	return msg.ContentMeta.Operation
}

//UpdateID returns message object updating its ID
func (msg *Message) UpdateID() *Message {
	msg.Header.ID = uuid.NewV4().String()
	return msg
}

// NewRawMessage returns a new raw message:
// model.NewRawMessage().BuildHeader().BuildRouter().FillBody()
func NewRawMessage() *Message {
	return &Message{}
}

// NewMessage returns a new basic message:
// model.NewMessage().BuildRouter().FillBody()
func NewMessage(parentID string) *Message {
	msg := &Message{}
	msg.Header.ID = uuid.NewV4().String()
	msg.Header.ParentID = parentID
	msg.Header.Timestamp = time.Now().UnixNano() / 1e6
	return msg
}

// Clone a message
// only update message id
func (msg *Message) Clone(message *Message) *Message {
	msgID := uuid.NewV4().String()
	return NewRawMessage().
		SetHeaderRaw(msgID, message.GetParentID(), message.GetTimestamp()).
		SetRouter(msg.GetRouter()).
		FillBody(message.GetContent())
}

// NewRespByMessage returns a new response message by a message received
func (msg *Message) NewRespByMessage(message *Message, self string, content interface{}) *Message {
	return NewMessage(message.GetID()).
		SetRouterRaw(self, message.GetSource(),message.IsBroadcast()).
		SetContentMeta(message.GetContentMeta()).
		FillBody(content)
}

// NewErrorMessage returns a new error message by a message received
func NewErrorMessage(message *Message, errContent string) *Message {
	meta := message.GetContentMeta()
	meta.Operation = ResponseErrorOperation
	return NewMessage(message.Header.ParentID).
		SetContentMeta(meta).
		FillBody(errContent)
}
