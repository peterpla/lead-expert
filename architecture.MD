# Architecture - MyLeadExperts

## An automated service to deliver tagged call transcripts from audio/video recordings

## Checklist to add a new pipeline stage

- Create a new folder under `cmd` with the same name as the service (using Go naming conventions, e.g., for `initial-request` service, the folder name would be `initialRequest`)
- Copy and paste `main.go` from `initialRequest`, modify:
  - in `init()` modify `logPrefix`
  - in `main()` modify `prefix`
  - in `taskHandler()` implement appropriate task handling for the new pipeline stage - **the heavy lifting**
- Copy and paste `app.yaml` from `initialRequest`, modify:
  - change `service:` to the new stage's service name (convention is to use the Cloud Tasks queue name but all lower case and with dashes between words)
  - under `env_variables:` change the *names* of the several `TASK_` lines and give them appropriate values
    - e.g., the `initial-request` service has env vars that begin with  `TASK_INITIAL_REQUEST`; change the names of all of those to `TASK_`your_service_name, and set the values appropriately
  - change the values of the `TASK_` env vars as needed
- in `pkg/config/config.go`:
  - at bottom, update the `Config struct` definition to add the "friendly name" for the env vars created above
  - at top, update the `[]binding`: for each env var, pair the friendly name just created with the env var name created earlier
- to run locally, update `.bash_profile` or equivalent with the env vars above
  - restart VC Code or equivalent after changing `.bash_profile` so those changes take effect
  - obviously the env var names and values in `app.yaml` and `.bash_profile` should match
- if needed, update `config.yaml` with configuration info that will be stored encrypted and read by `GetConfig()`
  - Ensure both `.gitignore` and `.gcloudignore` exclude `config.yaml`
  - The script `gupload_encrypted.sh` will encrypt `config.yaml` and copy it to Google Cloud Storage in the bucket specified by the `ENCRYPTED_BUCKET` env var
- update `_test.go` files including `pkg/config/config_test.go` to include the new `Config struct` members in `defaultResult`
- **IMPORTANT:** run `pkg/config/config_test.go` to ensure the friendly names and env var names all are correct. **Mistakes here are common sources of early deployment failures!**
- modify `gdeploy.sh` to deploy the new pipeline stage's service by adding the path to the service's `app.yaml`
- run `bash ./gdeploy.sh` to deploy all services
  - ... or copy and paste the `gsutil -m rm`... command to remove old files, and copy, paste and edit the `gcloud app deploy`... command to deploy only the new service

## Packages

``` text
cmd (things you start)
|__server
   |__main.go (HTTP server, minimal routing, request handler, etc.)
|__[each service (pipeline stage)]
   |__main.go (HTTP server, /task_handler, etc.)
pkg
|__appengine
|__check
|__config (process configuration inputs)
|__database
|__middleware
|__queue
|__request
|__serviceInfo
```

## GAE Service Implementation

Google App Engine services as defined in `cmd/*/app.yaml` files.

``` text
default (./cmd/server, app startup)
|__initial-request (./cmd/initialRequest/app.yaml, incoming HTTP requests)
|__service-dispatch (./cmd/serviceDispatch/app.yaml, dispatch request to preferred ML service)
|__transcription-gcp (./cmd/transcriptionGCP/app.yaml, submit to Google Speech-to-Text service)
|__transcription-complete (./cmd/transcriptionComplete/app.yaml, post-processing of all completed transcripts)
|__transcript-qa (./cmd/transcriptQA/app.yaml, perform QA on ML-generated transcript)
|__transcript-qa-complete (./cmd/transcriptQAComplete/app.yaml, post-processing of QA'd transcripts)
|__tagging (./cmd/tagging/app.yaml, dispatch transcript to preferred tagging service)
|__tagging-complete (./cmd/taggingComplete/app.yaml, post-processing of all completed tagging requests)
|__tagging-qa (./cmd/taggingQA/app.yaml, dispatch tagged transcripts for preferred QA service)
|__tagging-qa-complete (./cmd/taggingQAComplete/app.yaml, post-processing of all completed tagging QA)
|__completion-processing (./cmd/completionProcessing/app.yaml, final processing of fully-completed requests)
```

## Cloud Tasks Queues

Google Cloud Tasks queues and flow: *(see Airtable, "Flooring: Transcription Pipeline" for details)*

1. **InitialRequest**: tasks added by `default` service, handled by `initial-request` service implemented by `./cmd/initialRequest/main.go` and `/task_handler` endpoint
1. **ServiceDispatch**: tasks added by `initial-request` service, handled by `service-dispatch` service implemented by `./cmd/serviceDispatch/main.go` and `/task_handler` endpoint
1. **TranscriptionGCP**: tasks added by `service-dispatch` service, handled by `transcription-gcp` service implemented by `./cmd/transcriptionGCP/main.go` and `/task_handler` endpoint
1. **TranscriptionComplete**: tasks added by `transcription-gcp` service, handled by `transcription-complete` service implemented by `./cmd/transcriptionComplete/main.go` and `/task_handler` endpoint
1. **TranscriptQA**: tasks added by `transcription-complete` service, handled by `transcript-QA` service implemented by `./cmd/transcriptQA/main.go` and `/task_handler` endpoint
1. **TranscriptQAComplete**: tasks added by `transcript-QA` service, handled by `transcript-QA-complete` service  implemented by `./cmd/transcriptionQAComplete/main.go` and `/task_handler` endpoint
1. **Tagging**: tasks added by `transcript-QA-complete` service, handled by `tagging` service implemented by `./cmd/tagging/main.go` and `/task_handler` endpoint
1. **TaggingComplete**: tasks added by `tagging` service, handled by `tagging-complete` service implemented by `./cmd/taggingComplete/main.go` and `/task_handler` endpoint
1. **TaggingQA**: tasks added by `tagging-complete` service, handled by `tagging-QA` service implemented by `./cmd/taggingQA/main.go` and `/task_handler` endpoint
1. **TaggingQAComplete**: tasks added by `tagging-qa` service, handled by `tagging-qa-complete` service implemented by `./cmd/taggingQAComplete/main.go` and `/task_handler` endpoint
1. **CompletionProcessing**: tasks added by `tagging-qa-complete` service, handled by `completion-processing` service implemented by `./cmd/completionProcessing/main.go` and `/task_handler` endpoint

## Database Activity

Services that modify the database:

1. `default` service Create's an initial `Request` record in the collection specified by `DatabaseRequests` in `config.yaml.enc` (encrypted); currently `leadexperts-requests`.
2. `TranscriptionGDP` service Update's the current `Request` record in the database above, setting `WorkingTranscript` and `UpdatedAt` (and perhaps other fields).
3. `CompletionProcessing` service Update's the current `Request` record in the database above, processing `WorkingTranscript` to customer-ready form, saving the result as `FinalTranscript`, and setting `CompletedAt` (and perhaps other fields).

---

## --- old information follows, of limited value ---

## Domain-Driven Design artifacts

### Language

- **Actionable Sales Information**: data extracted during transcription processing that enables the customer to pursue sales opportunities
- **Billing**: record billable activities and periodically charge the customer, e.g., for transcriptions delivered and storage used
- **Customer**
- **External service**: e.g., Sonix.ai, Google Speech-to-Text, Dropbox
- **Media**: the audio or video file to be transcribed
- **Persistence**: storing work products for later use by the customer or us
- **Prep**: perform setup steps required for the next stage of request processing
- **Queue**: an ordered sequence of requests at various stages of processing
- **Reporting**: periodically providing information the customer wants, delivered as they prefer
- **Request**: a customer-initiated event to begin producing a transcription; may be initiated by an external service on behalf of the customer, e.g., a new Twilio call recording is available
- **Review**: a human corrects/enhances the transcript produced by an external service, improving the transcript to be more accurate and valuable
- **Reviewer**: a human that performs reviews
- **Transcript**: a text file with the written transcript produced from the provided media file
- **Validation**: confirm a request meets the requirements to proceed to the next stage of request processing

### Services

**Request**: create, enqueue, dequeue, process, complete ... a request

**Transcript**: create, review, tag, complete, deliver ... a transcript

**Reporting**: create, complete, deliver ... a report

### Value Objects

Customer, request, media file, transcript with tagging metadata

### Events

**Request**: rejected, created, added to queue, dispatched to worker, process step completed and removed from queue, retried, special handling required

**Transcript**: created, under review, review completed, transcripion completed, delivered to customer, special handling required

**Customer**: added, modified, transcription requested, transcript delivered, report delivered, service charge allocation added, billing event, external service resource billed

**External Services**: processing by service requested, processing completed, service resource consumed (capturing key metrics), service charge received, service charges matched to customer usage, service charges allocated to customers for billing

**Billing**: payment method established, payment method verified, external service consumed (e.g., seconds of processing), periodic charges calculated (e.g., GB of storage), customer payment initiated (e.g., credit card charged), customer payment received

### Repository

**Customer Repository**: information we need about the customer and their processing preferences. **Contains**: contact info, configuration info, billing info.

**Customer Artifact Repository**: enables the customer to review previously completed recordings, transcripts and reports. Enables us to revisit previously completed requests and work product to develop customized ML models, assess improvement by external services. **Contains**: original audio files; intermediate transcripts and tagging metadata (associated with original audio file); delivered transcripts and tagging metadata (associated with original audio file); reports.
