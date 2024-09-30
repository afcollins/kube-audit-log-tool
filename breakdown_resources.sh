#!/bin/bash -x

. common.sh

first_arg $1

welcome

FILTER_RESOURCE=$2
if [ "z"$2 == "z" ] ; then
	OUTPUT_RESOURCE=resource.uniq
	echo "Using the top three from $OUTPUT_RESOURCE"
	FILTER_RESOURCE=$( tail -3 $OUTPUT_RESOURCE | awk '{ print $2 }' | tr -d '"' )
fi

echo "breaking down resources ..."
for i in $FILTER_RESOURCE ; do

	echo "$i verbs ..."
	OUTPUT_VERB=resource_${i}.verb.uniq
	cat $FILE | jq --arg RESOURCE $i 'select( .objectRef.resource == $RESOURCE ) | .verb' | sort  | uniq -c | sort -n  > $OUTPUT_VERB
	tail -3 $OUTPUT_VERB
	echo

	echo "$i usernames ..."
	OUTPUT_USERNAME=resource_${i}.username.uniq
	cat $FILE | jq --arg RESOURCE $i 'select( .objectRef.resource == $RESOURCE ) | .user.username' | sort  | uniq -c | sort -n  > $OUTPUT_USERNAME
	tail -3 $OUTPUT_USERNAME
	echo

done
