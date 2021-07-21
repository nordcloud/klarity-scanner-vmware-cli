JIRA: https://nordcloud.atlassian.net/browse/{{repoLocation}}-TICKET-NR

# What

- What was done in this Pull Request
- What has not been done in this Pull Request (if work is split into multiple Pull Requests)
- How was it before
- Screenshots
- Any other information that may be useful for the dev to review Pull Request

## Testing

- [ ] Is this change covered by the unit tests?
- [ ] Is this change covered by the integration tests?
- [ ] Is this change covered by the automated acceptance tests? (if applicable)

## Compatibility

- [ ] Does this change maintain backward compatibility?

## Monitoring

- [ ] Will this change be covered by our existing monitoring?
      (including logs, metrics, datadog and rollbar integrations)
- [ ] Is this change compliant with currently provisioned resources and/or limits?
      (including CPU, memory, calls to other services, e.g. database performance)
- [ ] Can this change be deployed to Prod without triggering any alarms?
      (e.g. does it cause application downtime?)

## Rollout

- [ ] Can this change be merged immediately into the pipeline upon approval?
- [ ] Are all dependent changes already deployed to Prod?
- [ ] Can this change be rolled back without any issues after deployment to Prod?
      (e.g. large database migrations may be hard to rollback)

## Documentation

- [ ] Is the documentation up to date with this change?
