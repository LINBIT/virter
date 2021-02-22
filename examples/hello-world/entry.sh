#!/bin/sh
for h in $(echo "$TARGETS" | tr ',' '\n'); do
	ssh $h "echo hello world > /file"
done
