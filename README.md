# file-share

A simple http server to share files locally

It is a simple server to share files on your local network.

Features:

- It shows the current machine's IP address based URLs so that the other machines can reach it through browsers.
    - This can be opened by Ctrl+click on terminals that support this
    - Then a QR code can be shown from the browser if it supports it
- It accepts uploads so transfers are possible in both directions.
- If an argument is specified:
    - It uses the argument as the base directory
    - It required a specific prefix on the path generated randomly for minimal security. This is where a browser that supports QR code generation becomes useful, to prevent typing everything.
        - If this is not desired, one can `cd` to that directory and start it without an argument.
- Supports viewing and downloading links (marked `D` next to the size)
