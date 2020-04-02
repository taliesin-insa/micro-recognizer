# Micro-recognizer API
API for the microservice sending images to the recognizer in order to get suggestions for the text contained in these images.

## Home Link \[/recognizer\]
Simple method to test if the API is running correctly.

### \[GET\]
- Response 200 (text/plain)
    ~~~text
    [MICRO-RECOGNIZER] HomeLink joined
    ~~~

## \[/recognizer/sendImgs\]
Single request of the microservice.
1. Retrieves images from the database
2. Sends them to the distant recognizer
3. Gets suggestions of annotations provided by the recognizer in return
4. Sends theses suggestions to the database
5. Start again at step 1, until there isn't anymore images to annotate by the recognizer

### \[POST\]
No parameters required. This call launches the process described above. Since it may take a lot of time (especially if there are lots of images to annotate) it doesn't wait until the end of the process to send a response. By doing so, we avoid blocking the caller (since REST requests are synchronous).

Thus, if an error occurs, the caller won't have any feedback on it. To find error causes, please refer to the logs of the microservice.

- Response 202 => Accepted
