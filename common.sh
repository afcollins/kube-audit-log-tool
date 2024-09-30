
first_arg(){
	if [ "z"$1 == "z" ] ; then
		echo "Must provide a file as a first argument"
		exit 1
	fi
	FILE=$1
}

welcome(){
	echo "Start of file : $(head  -1 $FILE | jq '.stageTimestamp')"
	echo "  End of file : $(tail  -1 $FILE | jq '.stageTimestamp')"
}
