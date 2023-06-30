# Protocol Documentation
<a name="top"></a>






<a name="blipai-deckard-Deckard"></a>

### Deckard Service


| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Add | [AddRequest](#blipai-deckard-AddRequest) | [AddResponse](#blipai-deckard-AddResponse) | Adds messages to the queue. If any message already exists (same id and queue) it will be updated |
| Pull | [PullRequest](#blipai-deckard-PullRequest) | [PullResponse](#blipai-deckard-PullResponse) | Pulls messages from the queue |
| Ack | [AckRequest](#blipai-deckard-AckRequest) | [AckResponse](#blipai-deckard-AckResponse) | Acks a message, marking it as successfully processed |
| Nack | [AckRequest](#blipai-deckard-AckRequest) | [AckResponse](#blipai-deckard-AckResponse) | Nacks a message, marking it as not processed |
| Count | [CountRequest](#blipai-deckard-CountRequest) | [CountResponse](#blipai-deckard-CountResponse) | Counts number of messages on queue based on a filter request |
| Remove | [RemoveRequest](#blipai-deckard-RemoveRequest) | [RemoveResponse](#blipai-deckard-RemoveResponse) | Removes one or many messages from the queue based on its ids |
| Flush | [FlushRequest](#blipai-deckard-FlushRequest) | [FlushResponse](#blipai-deckard-FlushResponse) | Discards all deckard data from storage and cache. This request is only available if deckard instance is configured with MEMORY cache and storage. |
| GetById | [GetByIdRequest](#blipai-deckard-GetByIdRequest) | [GetByIdResponse](#blipai-deckard-GetByIdResponse) | Gets a message from a specific queue using its id CAUTION: this should be used mainly for diagnostics and debugging purposes since it will be direct operation on the storage system |
| EditQueue | [EditQueueRequest](#blipai-deckard-EditQueueRequest) | [EditQueueResponse](#blipai-deckard-EditQueueResponse) | Edits a queue configuration |
| GetQueue | [GetQueueRequest](#blipai-deckard-GetQueueRequest) | [GetQueueResponse](#blipai-deckard-GetQueueResponse) | Gets the current configuration for a queue |

 <!-- end services -->

## Messages


<a name="blipai-deckard-AckRequest"></a>

### AckRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | ID of the message |
| queue | [string](#string) |  | Queue where this message is stored |
| reason | [string](#string) |  | Reason of this result. Useful for audit, mostly on 'nack' signals. |
| score_subtract | [double](#double) |  | The value to subtract the score and increase final message score. For example if you want to make this message to have a better score you can add 10000 which will represent 10s of score benefit. If you want to penalize the message you can send a negative number.<br/><br/>IMPORTANT: The message will not be locked by, in the example, 10 seconds. This attribute is used only to increase or decrease the message priority in the priority queue.<br/><br/>This attribute is used only for ack requests and can't be used at the same time of 'lock_ms' attribute. |
| breakpoint | [string](#string) |  | Breakpoint to set for this message |
| lock_ms | [int64](#int64) |  | Time in milliseconds to lock a message before returning it to the queue. For nack requests the message will be locked before returning to first position in the priority queue. For ack requests the message will be locked before returning to last position in the priority queue.<br/><br/>IMPORTANT: The 'score_subtract' attribute will be ignored if this attribute is different than 0.<br/><br/>IMPORTANT: Deckard checks for locked messages in a 1-second delay meaning the lock have a second precision and not milliseconds. This field is in milliseconds because all scores and duration units on deckard are expressed in milliseconds. |
| removeMessage | [bool](#bool) |  | Whether the message should be removed when acked/nacked |






<a name="blipai-deckard-AckResponse"></a>

### AckResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| success | [bool](#bool) |  |  |
| removal_response | [RemoveResponse](#blipai-deckard-RemoveResponse) |  | The removal response if the message's removal was requested |






<a name="blipai-deckard-AddMessage"></a>

### AddMessage



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Unique id of this message |
| payload | [AddMessage.PayloadEntry](#blipai-deckard-AddMessage-PayloadEntry) | repeated | A payload map with formatted data to be stored and used by clients. |
| string_payload | [string](#string) |  | Non-formatted string payload. This field can be used to store simple string data instead of using the payload field. |
| metadata | [AddMessage.MetadataEntry](#blipai-deckard-AddMessage-MetadataEntry) | repeated | Metadata is a map of string to be used as a key-value store. It is a simple way to store data that is not part of the message payload. |
| queue | [string](#string) |  | Name of the queue to add this message<br/><br/>Suggestion: to better observability, provide the name of the application using colon as the separator. Example: <application_name>:<queue_name><br/><br/>You may also add a queue prefix to the queue name using two colons as the separator. Example: <application_name>:<queue_prefix>::<queue_name> |
| timeless | [bool](#bool) |  | Indicate this message will never expire and will only be deleted from the queue if explicitly removed. |
| ttl_minutes | [int64](#int64) |  | TTL is the amount of time in minutes this message will live in the queue. TTL is not a exact time, the message may live a little longer than the specified TTL. |
| description | [string](#string) |  | Description of the message, this should be used as a human readable string to be used in diagnostics. |






<a name="blipai-deckard-AddMessage-MetadataEntry"></a>

### AddMessage.MetadataEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="blipai-deckard-AddMessage-PayloadEntry"></a>

### AddMessage.PayloadEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [google.protobuf.Any](#google-protobuf-Any) |  |  |






<a name="blipai-deckard-AddRequest"></a>

### AddRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [AddMessage](#blipai-deckard-AddMessage) | repeated | List of messages to be added |






<a name="blipai-deckard-AddResponse"></a>

### AddResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| created_count | [int64](#int64) |  | Number of created messages |
| updated_count | [int64](#int64) |  | Number of updated messages |






<a name="blipai-deckard-CountRequest"></a>

### CountRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| queue | [string](#string) |  |  |






<a name="blipai-deckard-CountResponse"></a>

### CountResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| count | [int64](#int64) |  |  |






<a name="blipai-deckard-EditQueueRequest"></a>

### EditQueueRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| queue | [string](#string) |  | Name of the queue to be updated This includes all prefixes and suffixes |
| configuration | [QueueConfiguration](#blipai-deckard-QueueConfiguration) |  | Configuration to apply to the queue. It will always update the queue with the newer configuration. Only available fields will be updated, meaning that previously configured attributes will not be change unless you explicit set it. If you want to change a configuration to its default value, manually set it to its default value following each field documentation. |






<a name="blipai-deckard-EditQueueResponse"></a>

### EditQueueResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| queue | [string](#string) |  | Name of the queue |
| success | [bool](#bool) |  |  |






<a name="blipai-deckard-FlushRequest"></a>

### FlushRequest







<a name="blipai-deckard-FlushResponse"></a>

### FlushResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| success | [bool](#bool) |  |  |






<a name="blipai-deckard-GetByIdRequest"></a>

### GetByIdRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| queue | [string](#string) |  | Name of the queue to get a message |
| id | [string](#string) |  | The message id |






<a name="blipai-deckard-GetByIdResponse"></a>

### GetByIdResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| message | [Message](#blipai-deckard-Message) |  |  |
| human_readable_payload | [GetByIdResponse.HumanReadablePayloadEntry](#blipai-deckard-GetByIdResponse-HumanReadablePayloadEntry) | repeated | A human readable string data map of the message's payload.<br/><br/>This represents the payload map as a JSON string representation, useful for debugging and diagnostics |
| found | [bool](#bool) |  |  |






<a name="blipai-deckard-GetByIdResponse-HumanReadablePayloadEntry"></a>

### GetByIdResponse.HumanReadablePayloadEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="blipai-deckard-GetQueueRequest"></a>

### GetQueueRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| queue | [string](#string) |  | Name of the queue to be updated This includes all prefixes and suffixes |






<a name="blipai-deckard-GetQueueResponse"></a>

### GetQueueResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| queue | [string](#string) |  | Name of the queue |
| configuration | [QueueConfiguration](#blipai-deckard-QueueConfiguration) |  | Configuration of the queue |






<a name="blipai-deckard-Message"></a>

### Message



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | ID is an unique identifier inside a queue. Any message with the same id and queue will be considered the same message. |
| description | [string](#string) |  | Description of the message, this should be used as a human readable string to be used in diagnostics. |
| queue | [string](#string) |  | Full name of the queue this message belongs (including any prefixes) |
| payload | [Message.PayloadEntry](#blipai-deckard-Message-PayloadEntry) | repeated | A payload map with formatted data to be stored and used by clients. |
| metadata | [Message.MetadataEntry](#blipai-deckard-Message-MetadataEntry) | repeated | Metadata is a map of string to be used as a key-value store. It is a simple way to store data that is not part of the message payload. |
| string_payload | [string](#string) |  | Message string payload. Is responsibility of the caller to know how to encode/decode to a useful format for its purpose. This field can be used to store simple string data instead of using the payload field. |
| score | [double](#double) |  | Score is the priority this message currently have in the queue. |
| breakpoint | [string](#string) |  | Breakpoint is a field to be used as an auxiliar field for some specific use cases.<br/><br/>For example if you need to keep a record of the last result processing a message, use this field like iteracting with a pagination system. |






<a name="blipai-deckard-Message-MetadataEntry"></a>

### Message.MetadataEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="blipai-deckard-Message-PayloadEntry"></a>

### Message.PayloadEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [google.protobuf.Any](#google-protobuf-Any) |  |  |






<a name="blipai-deckard-PullRequest"></a>

### PullRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| queue | [string](#string) |  | Full name of the queue to pull messages (including any prefixes) |
| amount | [int32](#int32) |  | Number of messages to fetch. Caution: as greater the amount, as more time it will take to process the request. Max value is 1000 and the default value is 1 |
| score_filter | [int64](#int64) |  | Number to subtract to the current time to filter the max score to return. Useful to not return a message just moments after it was last used.<br/><br/>For example if in your queue the score is only based on the time (always acking with score_subtract as 0), this parameter will be the number of milliseconds since the message's last usage. |






<a name="blipai-deckard-PullResponse"></a>

### PullResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| messages | [Message](#blipai-deckard-Message) | repeated | List of returned messages |






<a name="blipai-deckard-QueueConfiguration"></a>

### QueueConfiguration
The queue configuration does not change instantly and can take up to 10 minutes to complete update.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| max_elements | [int64](#int64) |  | Number of max elements the queue can have.<br/><br/>To apply a max elements to a queue, set a value greater than 0. To remove the max elements from a queue, set the value to -1. 0 will be always ignored and the queue will not be updated.<br/><br/>All queues are unlimited by default. |






<a name="blipai-deckard-RemoveRequest"></a>

### RemoveRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ids | [string](#string) | repeated |  |
| queue | [string](#string) |  | Name of the queue to remove message Provide the name of the application as a prefix using colon as the separator. Example: <application_name>:<queue_name> |






<a name="blipai-deckard-RemoveResponse"></a>

### RemoveResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| cacheRemoved | [int64](#int64) |  |  |
| storageRemoved | [int64](#int64) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="google.protobuf.Any">[google.protobuf.Any](https://protobuf.dev/programming-guides/proto3/#any)</a> | The Any message type lets you use messages as embedded types without having their .proto definition. An Any contains an arbitrary serialized message as bytes, along with a URL that acts as a globally unique identifier for and resolves to that message's type. | [documentation](https://protobuf.dev/reference/cpp/cpp-generated/#any) | [documentation](https://protobuf.dev/reference/java/java-generated/#any) | [documentation](https://protobuf.dev/reference/python/python-generated/#any) | [documentation](https://pkg.go.dev/google.golang.org/protobuf/types/known/anypb) | [documentation](https://learn.microsoft.com/en-us/dotnet/architecture/grpc-for-wcf-developers/protobuf-any-oneof#any) | [documentation](https://github.com/protocolbuffers/protobuf/blob/main/php/src/Google/Protobuf/Any.php) | [documentation](https://cloud.google.com/ruby/docs/reference/google-cloud-container_analysis-v1/latest/Google-Protobuf-Any) |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

