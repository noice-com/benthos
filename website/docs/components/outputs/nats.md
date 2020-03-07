---
title: nats
type: output
---

<!--
     THIS FILE IS AUTOGENERATED!

     To make changes please edit the contents of:
     lib/output/nats.go
-->


Publish to an NATS subject.

```yaml
# Config fields, showing default values
output:
  nats:
    urls:
    - nats://127.0.0.1:4222
    subject: benthos_messages
    max_in_flight: 1
```

This output will interpolate functions within the subject field, you
can find a list of functions [here](/docs/configuration/interpolation#functions).

## Performance

This output benefits from sending multiple messages in flight in parallel for
improved performance. You can tune the max number of in flight messages with the
field `max_in_flight`.

## Fields

### `urls`

A list of URLs to connect to. If an item of the list contains commas it will be expanded into multiple URLs.


Type: `array`  
Default: `["nats://127.0.0.1:4222"]`  

### `subject`

The subject to publish to.
This field supports [interpolation functions](/docs/configuration/interpolation#functions).


Type: `string`  
Default: `"benthos_messages"`  

### `max_in_flight`

The maximum number of messages to have in flight at a given time. Increase this to improve throughput.


Type: `number`  
Default: `1`  

