# micro-recognizer

Microservice that calls the daemon running on a distant server, to get automatic transcriptions of images, via a text recognizer.

## Exposed REST API

See [the API specification](api.md).

## CronJob
A CronJob was created in order to trigger regularly the main request of this microservice.
This cron is here in case that the microservice's pod is killed during a process of suggestion by the recognizer. Having this cron, the new instance of the microservice will resume fetching and sending images to the recognizer.

To install the cron on kubernetes, clone this repo on inky, then use the following command:
`k8s create -f micro-recognizer/cronjob-recognizer.yml`.

To delete the running cron, use:
`k8s delete cronjob micro-recognizer-waker`

Pods created by Kubernetes when the cron is executed are deleted automatically if the cron succeeds. The 3 last failed pods are kept, for debug purpose.

If you want to be sure that the cron is executed correctly, since you won't see an associated pod in the list of pods, you can use the following command, to track the activations of the cron:
`k8s get jobs --watch`

## Commits
The title of a commit must follow this pattern : \<type>(\<scope>): \<subject>

### Type
Commits must specify their type among the following:
* **build**: changes that affect the build system or external dependencies
* **docs**: documentation only changes
* **log**: changes in the logging messages
* **feat**: a new feature
* **fix**: a bug fix
* **perf**: a code change that improves performance
* **refactor**: modifications of code without adding features nor bugs (rename, white-space, etc.)
* **style**: CSS, layout modifications or console prints
* **test**: tests or corrections of existing tests
* **ci**: changes to our CI configuration


### Scope
Your commits name should also precise which part of the project they concern. You can do so by naming them using the following scopes:
* api
* general
* cron
