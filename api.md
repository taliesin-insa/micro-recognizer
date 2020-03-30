# Micro-recognizer API
API for the microservice sending images to the recognizer in order to get suggestions for the text contained in these images.

## Home Link \[/recognizer\]
Simple method to test if the API is running correctly.

### \[GET\]
- Response 200 (text/plain)
    ~~~text
    [MICRO-RECOGNIZER] HomeLink joined
    ~~~

## Retrieve transcriptions for images \[/recognizer/sendImgs\]
Main request.

### \[POST\]
No parameters required.

- Response 200 -> Ok if everything goes right

- Response 500 -> InternalServerError (text/plain)
    - Body contains a description of the error.
