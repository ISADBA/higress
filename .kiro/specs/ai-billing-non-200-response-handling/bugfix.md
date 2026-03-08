# Bugfix Requirements Document

## Introduction

The ai-billing plugin currently fails to properly handle non-200 HTTP responses from AI providers (Gemini, OpenAI, etc.). When providers return 4xx or 5xx error status codes, the plugin correctly skips billing but breaks the HTTP/2 stream, causing clients to receive an ungraceful `INTERNAL_ERROR (err 2)` instead of the provider's original error message and status code. This prevents clients from understanding what went wrong and makes debugging difficult.

This bugfix ensures that non-200 responses are properly passed through to clients while maintaining the correct billing behavior (no cost deduction for errors).

## Bug Analysis

### Current Behavior (Defect)

1.1 WHEN an AI provider returns a 4xx HTTP status code (client error) THEN the system logs the skip billing message but returns HTTP/2 INTERNAL_ERROR to the client instead of the provider's original error response

1.2 WHEN an AI provider returns a 5xx HTTP status code (server error) THEN the system logs the skip billing message but returns HTTP/2 INTERNAL_ERROR to the client instead of the provider's original error response

1.3 WHEN an AI provider returns any non-200 HTTP status code THEN the client receives `curl: (92) HTTP/2 stream 1 was not closed cleanly: INTERNAL_ERROR (err 2)` instead of seeing the provider's error message and status code

1.4 WHEN a non-200 response occurs THEN the system logs only minimal information (`[ai-billing] skipping billing for non-200 response: status=421`) without tenant, consumer, provider, or model context

### Expected Behavior (Correct)

2.1 WHEN an AI provider returns a 4xx HTTP status code THEN the system SHALL skip billing AND pass through the provider's original error response with the correct status code to the client AND log at Info level with full context (tenant, consumer, provider, model, status)

2.2 WHEN an AI provider returns a 5xx HTTP status code THEN the system SHALL skip billing AND pass through the provider's original error response with the correct status code to the client AND log at Warn level with full context (tenant, consumer, provider, model, status)

2.3 WHEN an AI provider returns any other non-200 HTTP status code THEN the system SHALL skip billing AND pass through the provider's original error response with the correct status code to the client AND log at Debug level with full context

2.4 WHEN a non-200 response is passed through THEN the client SHALL receive the provider's original HTTP status code and error message body without HTTP/2 stream errors

### Unchanged Behavior (Regression Prevention)

3.1 WHEN an AI provider returns a 200 HTTP status code THEN the system SHALL CONTINUE TO process billing normally and pass through the successful response

3.2 WHEN billing information is successfully extracted from a 200 response THEN the system SHALL CONTINUE TO deduct costs and send billing requests to the cost service

3.3 WHEN the response stream is processed THEN the system SHALL CONTINUE TO extract request IDs, provider information, and usage metrics correctly

3.4 WHEN tenant and consumer information is available THEN the system SHALL CONTINUE TO include it in billing requests for 200 responses
