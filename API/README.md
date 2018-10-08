# API

Submodule for working with different sources of memes. All of them must satisfy following API:

### Get memes
* `GET /memes`

Response:

`message Meme {
  int32 id = 1;
  string meme_id = 2;
  string public = 3;
  string platform = 4;
  repeated string pictures = 5;
  string Description = 6;
  int32 likes = 7;
  int32 reposts = 8;
  int32 views = 9;
  int32 comments = 10;
  int64 time = 11;
}`

### Get memes for specified amount of time

* `GET /memes/from`

Same respose as for Get memes API call.


Services have 2 interfaces - gRPC (port in config) and HTTP REST (port in config + 1).
Services are not required to store information in DB, that is work for Core.
