#!/bin/bash

. common.sh

first_arg $1

welcome

FILTER_VERB=$2
if [ "z"$2 == "z" ] ; then
	OUTPUT_VERB=verb.uniq
	echo "Using the top three from $OUTPUT_VERB"
	FILTER_VERB=$( tail -3 $OUTPUT_VERB | awk '{ print $2 }' | tr -d '"' )
fi

echo "breaking down verbs ..."
OUTPUT_VERB=verb.uniq
for i in $FILTER_VERB ; do

	echo "$i resources ..."
	OUTPUT_RESOURCE=verb_${i}.resource.uniq
	cat $FILE | jq --arg VERB $i 'select( .verb == $VERB ) | .objectRef.resource' | sort  | uniq -c | sort -n  > $OUTPUT_RESOURCE
	tail -3 $OUTPUT_RESOURCE
	echo

	echo "$i usernames ..."
	OUTPUT_USERNAME=verb_${i}.username.uniq
	cat $FILE | jq --arg VERB $i 'select( .verb == $VERB ) | .user.username' | sort  | uniq -c | sort -n  > $OUTPUT_USERNAME
	tail -3 $OUTPUT_USERNAME
	echo

done
