# kytheron

### Quickstart 

First, we'll start the Kytheron server
``` 
docker compose up -d 
go run cmd/kytheron-db/* migrate 
go run cmd/kytheron/*.go -c samples/config.yaml
```

This will run the following systems
 - Postgres database 
 - Loki logging system 
 - Single node Kafka cluster
 - Kytheron server 
 - AIO processing component 

> Note: The topic subscriptions may fail as the topics
> aren't created initially. You may need to restart Kytheron after 
> producing some logs 

#### Produce some logs 

You can produce some Cloudtrail logs for testing using 
`kytheron-plugin-file`. In a separate directory

``` 
git clone git@github.com:kytheron-org/kytheron-plugin-file 
cd kytheron-plugin-file
touch test.txt 
export KYTHERON_URL=localhost:8000
go run *.go --file test.txt

# In another terminal, dump a cloudtrail record to it 
echo '{"Records":[{"userAgent":"[S3Console/0.4]","eventID":"3038ebd2-c98a-4c65-9b6e-e22506292313","userIdentity":{"type":"Root","principalId":"811596193553","arn":"arn:aws:iam::811596193553:root","accountId":"811596193553","sessionContext":{"attributes":{"mfaAuthenticated":"false","creationDate":"2017-02-12T19:57:05Z"}}},"eventType":"AwsApiCall","sourceIPAddress":"255.253.125.115","eventName":"ListBuckets","eventSource":"s3.amazonaws.com","recipientAccountId":"811596193553","requestParameters":null,"awsRegion":"us-east-1","requestID":"83A6C73FE87F51FF","responseElements":null,"eventVersion":"1.04","eventTime":"2017-02-12T19:57:06Z"}]}' >> test.txt
```

You should see the following outputs
``` 
# Server received the message
message received
{"data":"eyJSZWNvcmRzIjpbeyJ1c2VyQWdlbnQiOiJbUzNDb25zb2xlLzAuNF0iLCJldmVudElEIjoiMzAzOGViZDItYzk4YS00YzY1LTliNmUtZTIyNTA2MjkyMzEzIiwidXNlcklkZW50aXR5Ijp7InR5cGUiOiJSb290IiwicHJpbmNpcGFsSWQiOiI4MTE1OTYxOTM1NTMiLCJhcm4iOiJhcm46YXdzOmlhbTo6ODExNTk2MTkzNTUzOnJvb3QiLCJhY2NvdW50SWQiOiI4MTE1OTYxOTM1NTMiLCJzZXNzaW9uQ29udGV4dCI6eyJhdHRyaWJ1dGVzIjp7Im1mYUF1dGhlbnRpY2F0ZWQiOiJmYWxzZSIsImNyZWF0aW9uRGF0ZSI6IjIwMTctMDItMTJUMTk6NTc6MDVaIn19fSwiZXZlbnRUeXBlIjoiQXdzQXBpQ2FsbCIsInNvdXJjZUlQQWRkcmVzcyI6IjI1NS4yNTMuMTI1LjExNSIsImV2ZW50TmFtZSI6Ikxpc3RCdWNrZXRzIiwiZXZlbnRTb3VyY2UiOiJzMy5hbWF6b25hd3MuY29tIiwicmVjaXBpZW50QWNjb3VudElkIjoiODExNTk2MTkzNTUzIiwicmVxdWVzdFBhcmFtZXRlcnMiOm51bGwsImF3c1JlZ2lvbiI6InVzLWVhc3QtMSIsInJlcXVlc3RJRCI6IjgzQTZDNzNGRTg3RjUxRkYiLCJyZXNwb25zZUVsZW1lbnRzIjpudWxsLCJldmVudFZlcnNpb24iOiIxLjA0IiwiZXZlbnRUaW1lIjoiMjAxNy0wMi0xMlQxOTo1NzowNloifV19"}

# After placement on the ingest topic, the consumer is processing it
Message on ingest ingest[0]@102: {"data":"eyJSZWNvcmRzIjpbeyJ1c2VyQWdlbnQiOiJbUzNDb25zb2xlLzAuNF0iLCJldmVudElEIjoiMzAzOGViZDItYzk4YS00YzY1LTliNmUtZTIyNTA2MjkyMzEzIiwidXNlcklkZW50aXR5Ijp7InR5cGUiOiJSb290IiwicHJpbmNpcGFsSWQiOiI4MTE1OTYxOTM1NTMiLCJhcm4iOiJhcm46YXdzOmlhbTo6ODExNTk2MTkzNTUzOnJvb3QiLCJhY2NvdW50SWQiOiI4MTE1OTYxOTM1NTMiLCJzZXNzaW9uQ29udGV4dCI6eyJhdHRyaWJ1dGVzIjp7Im1mYUF1dGhlbnRpY2F0ZWQiOiJmYWxzZSIsImNyZWF0aW9uRGF0ZSI6IjIwMTctMDItMTJUMTk6NTc6MDVaIn19fSwiZXZlbnRUeXBlIjoiQXdzQXBpQ2FsbCIsInNvdXJjZUlQQWRkcmVzcyI6IjI1NS4yNTMuMTI1LjExNSIsImV2ZW50TmFtZSI6Ikxpc3RCdWNrZXRzIiwiZXZlbnRTb3VyY2UiOiJzMy5hbWF6b25hd3MuY29tIiwicmVjaXBpZW50QWNjb3VudElkIjoiODExNTk2MTkzNTUzIiwicmVxdWVzdFBhcmFtZXRlcnMiOm51bGwsImF3c1JlZ2lvbiI6InVzLWVhc3QtMSIsInJlcXVlc3RJRCI6IjgzQTZDNzNGRTg3RjUxRkYiLCJyZXNwb25zZUVsZW1lbnRzIjpudWxsLCJldmVudFZlcnNpb24iOiIxLjA0IiwiZXZlbnRUaW1lIjoiMjAxNy0wMi0xMlQxOTo1NzowNloifV19"}

# Print out the parsed logs
{"Records":[{"userAgent":"[S3Console/0.4]","eventID":"3038ebd2-c98a-4c65-9b6e-e22506292313","userIdentity":{"type":"Root","principalId":"811596193553","arn":"arn:aws:iam::811596193553:root","accountId":"811596193553","sessionContext":{"attributes":{"mfaAuthenticated":"false","creationDate":"2017-02-12T19:57:05Z"}}},"eventType":"AwsApiCall","sourceIPAddress":"255.253.125.115","eventName":"ListBuckets","eventSource":"s3.amazonaws.com","recipientAccountId":"811596193553","requestParameters":null,"awsRegion":"us-east-1","requestID":"83A6C73FE87F51FF","responseElements":null,"eventVersion":"1.04","eventTime":"2017-02-12T19:57:06Z"}]}
received parsed log
{"source_type":"cloudtrail","source_name":"TBD","data":"eyJhd3NSZWdpb24iOiJ1cy1lYXN0LTEiLCJldmVudElEIjoiMzAzOGViZDItYzk4YS00YzY1LTliNmUtZTIyNTA2MjkyMzEzIiwiZXZlbnROYW1lIjoiTGlzdEJ1Y2tldHMiLCJldmVudFNvdXJjZSI6InMzLmFtYXpvbmF3cy5jb20iLCJldmVudFRpbWUiOiIyMDE3LTAyLTEyVDE5OjU3OjA2WiIsImV2ZW50VHlwZSI6IkF3c0FwaUNhbGwiLCJldmVudFZlcnNpb24iOiIxLjA0IiwicmVjaXBpZW50QWNjb3VudElkIjoiODExNTk2MTkzNTUzIiwicmVxdWVzdElEIjoiODNBNkM3M0ZFODdGNTFGRiIsInJlcXVlc3RQYXJhbWV0ZXJzIjpudWxsLCJyZXNwb25zZUVsZW1lbnRzIjpudWxsLCJzb3VyY2VJUEFkZHJlc3MiOiIyNTUuMjUzLjEyNS4xMTUiLCJ1c2VyQWdlbnQiOiJbUzNDb25zb2xlLzAuNF0iLCJ1c2VySWRlbnRpdHkiOnsiYWNjb3VudElkIjoiODExNTk2MTkzNTUzIiwiYXJuIjoiYXJuOmF3czppYW06OjgxMTU5NjE5MzU1Mzpyb290IiwicHJpbmNpcGFsSWQiOiI4MTE1OTYxOTM1NTMiLCJzZXNzaW9uQ29udGV4dCI6eyJhdHRyaWJ1dGVzIjp7ImNyZWF0aW9uRGF0ZSI6IjIwMTctMDItMTJUMTk6NTc6MDVaIiwibWZhQXV0aGVudGljYXRlZCI6ImZhbHNlIn19LCJ0eXBlIjoiUm9vdCJ9fQ=="}

# The parsed logs emit to the 'parsed' Kafka topic
Producing message
Parsed log put on topic successfully

# We get a message for each cloudtrail 'record', and that stream completed
Received ROF, closing stream

# 'parsed' topic has received the message on its consumer
# from here, we would handle storage, evaluation, etc
Message on parsed parsed[0]@16: {"source_type":"cloudtrail","source_name":"TBD","data":"eyJhd3NSZWdpb24iOiJ1cy1lYXN0LTEiLCJldmVudElEIjoiMzAzOGViZDItYzk4YS00YzY1LTliNmUtZTIyNTA2MjkyMzEzIiwiZXZlbnROYW1lIjoiTGlzdEJ1Y2tldHMiLCJldmVudFNvdXJjZSI6InMzLmFtYXpvbmF3cy5jb20iLCJldmVudFRpbWUiOiIyMDE3LTAyLTEyVDE5OjU3OjA2WiIsImV2ZW50VHlwZSI6IkF3c0FwaUNhbGwiLCJldmVudFZlcnNpb24iOiIxLjA0IiwicmVjaXBpZW50QWNjb3VudElkIjoiODExNTk2MTkzNTUzIiwicmVxdWVzdElEIjoiODNBNkM3M0ZFODdGNTFGRiIsInJlcXVlc3RQYXJhbWV0ZXJzIjpudWxsLCJyZXNwb25zZUVsZW1lbnRzIjpudWxsLCJzb3VyY2VJUEFkZHJlc3MiOiIyNTUuMjUzLjEyNS4xMTUiLCJ1c2VyQWdlbnQiOiJbUzNDb25zb2xlLzAuNF0iLCJ1c2VySWRlbnRpdHkiOnsiYWNjb3VudElkIjoiODExNTk2MTkzNTUzIiwiYXJuIjoiYXJuOmF3czppYW06OjgxMTU5NjE5MzU1Mzpyb290IiwicHJpbmNpcGFsSWQiOiI4MTE1OTYxOTM1NTMiLCJzZXNzaW9uQ29udGV4dCI6eyJhdHRyaWJ1dGVzIjp7ImNyZWF0aW9uRGF0ZSI6IjIwMTctMDItMTJUMTk6NTc6MDVaIiwibWZhQXV0aGVudGljYXRlZCI6ImZhbHNlIn19LCJ0eXBlIjoiUm9vdCJ9fQ=="}

```