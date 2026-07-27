# xhttp-box Copilot instructions

xhttp-box is an independent, unofficial derivative of SagerNet/sing-box that
adds and maintains Xray-compatible XHTTP transport support.

When adapting upstream changes:

- Preserve the repository's independent name, attribution, and disclaimer.
- Preserve `transport/v2rayxhttp`, its option types, transport registration,
  English and Chinese documentation, and all XHTTP tests.
- Maintain bidirectional Xray-core interoperability, REALITY/uTLS support,
  optional HTTP/3 support, XMUX behavior, and lifecycle cleanup.
- Prefer upstream behavior outside project-specific XHTTP code.
- Make the smallest compatibility change necessary and avoid unrelated
  refactoring.
- Never weaken, delete, or skip tests merely to make an upstream sync pass.
- Run the normal, race, `with_quic`, and `with_utls` XHTTP test variants after
  modifying XHTTP code.
