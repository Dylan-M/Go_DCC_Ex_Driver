# Capability discovery and consist foundation

This foundation partially addresses [#10](https://github.com/Dylan-M/Go_DCC_Ex_Driver/issues/10).
It does not complete the consist builder or establish firmware support for
intelligent front/rear lighting.

## Current behavior

- Desktop Power Throttle mode has a Consists tab explaining its availability.
- Engineer mode and Android do not expose this administrative tab.
- The existing connection handshake is unchanged. No guessed discovery command,
  consist operation, decoder write, or lighting operation is sent.
- Uppercase `<X>` has a typed `protocol.CommandRejected` event and remains visible
  in the console. Lowercase `<x>` is not interpreted as that rejection.
- Capability state is connection-specific, not saved in the preferences database.
  Controller snapshots detach capability metadata; disconnecting clears it.

## Transport-independent contracts

`dccex/capabilities` provides a catalog with a response schema version and
independently versioned feature IDs, descriptions, and descriptive metadata.
Unknown features and versions can be retained without granting support.
Consumers require an exact schema and feature version they understand. Metadata
does not authorize loading or executing plugins.

The discovery state machine distinguishes not requested, pending, available,
unsupported query, inconclusive timeout, and invalid response. A valid empty
catalog means discovery works but advertises no extensions.

Only a pending, attributed query may consume a rejection. `<X>` alone cannot
identify which request failed, and it also represents malformed parameters or
other command failures. Attempt tokens protect against stale local callbacks;
they are not wire-level request IDs. Timeout or invalid data never enables
extensions. Replies at or after the deadline cannot complete an expired query.

## Remaining integration

Before enabling live discovery, agree on the read-only query command, response
encoding, schema version, capability IDs, and version compatibility rules with
the custom firmware. No name or version suffix is required to probe discovery.
The Go model is an internal contract, not a selected wire format.

The future session adapter must isolate discovery from unrelated requests and
polling, attribute responses, enforce a deadline, and handle transport failures
and reconnects. Merely changing controller status is not sufficient. Queries
must not modify a layout to test support, and no discovery result should be
restored from another station or previous connection.

Capability-specific support must then be verified before implementing consist
creation/deletion, locomotive orientation and front/rear roles, intelligent
lighting, and station-confirmed operating state. A basic consist response or a
firmware version number does not imply intelligent-lighting support.
