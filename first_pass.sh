#!/bin/bash

. common.sh

first_arg $1

welcome

echo "verbs ..."
OUTPUT_VERB=verb.uniq
cat $FILE | jq '.verb' | sort  | uniq -c | sort -n  > $OUTPUT_VERB

echo "resources ..."
OUTPUT_RESOURCE=resource.uniq
cat $FILE | jq '.objectRef.resource' | sort  | uniq -c | sort -n  > $OUTPUT_RESOURCE

echo "usernames ..."
OUTPUT_USERNAME=username.uniq
cat $FILE | jq '.user.username' | sort  | uniq -c | sort -n  > $OUTPUT_USERNAME

echo "minutes ..."
OUTPUT_MINUTES=stageTimestamp.minutes.uniq
cat $FILE | jq '.stageTimestamp' | cut -d ":" -f 1-2 | sort | uniq -c > $OUTPUT_MINUTES
sort -n $OUTPUT_MINUTES > ${OUTPUT_MINUTES}.sorted

echo "seconds ..."
OUTPUT_SECONDS=stageTimestamp.seconds.uniq
cat $FILE | jq '.stageTimestamp' | cut -d "." -f 1 | sort | uniq -c > $OUTPUT_SECONDS
sort -n $OUTPUT_SECONDS > ${OUTPUT_SECONDS}.sorted

echo "pod-names ..."
OUTPUT_PODNAME=pod-name.uniq
cat $FILE | jq '.user.extra."authentication.kubernetes.io/pod-name"[0]' | sort | uniq -c | sort -n > $OUTPUT_PODNAME

# print a small report

tail -3 $OUTPUT_VERB $OUTPUT_RESOURCE $OUTPUT_USERNAME $OUTPUT_PODNAME ${OUTPUT_SECONDS}.sorted ${OUTPUT_MINUTES}.sorted
