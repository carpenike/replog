/**
 * RepLog WebAuthn/Passkey support.
 *
 * Provides two flows:
 *   1. Registration — register a new passkey (Face ID / Touch ID) for the current user.
 *   2. Login — authenticate with a stored passkey (discoverable / usernameless).
 *
 * All server communication uses JSON endpoints under /passkeys/*.
 */
(function () {
    "use strict";

    // CSRF token helper — uses the global RepLog.csrfToken() defined in base layout.
    function csrfToken() {
        return (window.RepLog && RepLog.csrfToken) ? RepLog.csrfToken() : "";
    }

    // Base64URL encode/decode helpers for WebAuthn binary fields.
    function bufferToBase64url(buffer) {
        var bytes = new Uint8Array(buffer);
        var str = "";
        for (var i = 0; i < bytes.byteLength; i++) {
            str += String.fromCharCode(bytes[i]);
        }
        return btoa(str).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
    }

    function base64urlToBuffer(base64url) {
        var base64 = base64url.replace(/-/g, "+").replace(/_/g, "/");
        while (base64.length % 4) {
            base64 += "=";
        }
        var binary = atob(base64);
        var bytes = new Uint8Array(binary.length);
        for (var i = 0; i < binary.length; i++) {
            bytes[i] = binary.charCodeAt(i);
        }
        return bytes.buffer;
    }

    /**
     * Register a new passkey for the current logged-in user.
     * Called from the preferences page or user edit form.
     */
    async function registerPasskey(labelInputId, statusId) {
        var statusEl = document.getElementById(statusId);
        var labelInput = document.getElementById(labelInputId);
        var label = labelInput ? labelInput.value.trim() : "";

        if (statusEl) {
            statusEl.textContent = "Starting registration…";
            statusEl.className = "passkey-status";
        }

        try {
            // Store label on server before ceremony.
            if (label) {
                await fetch("/passkeys/register/label", {
                    method: "POST",
                    headers: { "Content-Type": "application/x-www-form-urlencoded", "X-CSRF-Token": csrfToken() },
                    body: "label=" + encodeURIComponent(label),
                });
            }

            // Step 1: Get creation options from server.
            var beginResp = await fetch("/passkeys/register/begin");
            if (!beginResp.ok) {
                throw new Error("Server error: " + beginResp.status);
            }
            var options = await beginResp.json();

            // Decode binary fields for the browser API.
            options.publicKey.challenge = base64urlToBuffer(options.publicKey.challenge);
            options.publicKey.user.id = base64urlToBuffer(options.publicKey.user.id);
            if (options.publicKey.excludeCredentials) {
                options.publicKey.excludeCredentials = options.publicKey.excludeCredentials.map(function (c) {
                    c.id = base64urlToBuffer(c.id);
                    return c;
                });
            }

            if (statusEl) {
                statusEl.textContent = "Waiting for biometric…";
            }

            // Step 2: Prompt user for biometric / security key.
            var credential = await navigator.credentials.create(options);

            // Step 3: Encode response for server.
            var attestationResponse = credential.response;
            var body = JSON.stringify({
                id: credential.id,
                rawId: bufferToBase64url(credential.rawId),
                type: credential.type,
                response: {
                    attestationObject: bufferToBase64url(attestationResponse.attestationObject),
                    clientDataJSON: bufferToBase64url(attestationResponse.clientDataJSON),
                },
            });

            // Step 4: Send to server.
            var finishResp = await fetch("/passkeys/register/finish", {
                method: "POST",
                headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() },
                body: body,
            });

            if (!finishResp.ok) {
                var err = await finishResp.json();
                throw new Error(err.error || "Registration failed");
            }

            if (statusEl) {
                statusEl.textContent = "Passkey registered successfully!";
                statusEl.className = "passkey-status success";
            }

            // Reload the page to show the new credential in the list.
            setTimeout(function () { window.location.reload(); }, 1000);
        } catch (e) {
            if (e.name === "NotAllowedError") {
                if (statusEl) {
                    statusEl.textContent = "Registration cancelled.";
                    statusEl.className = "passkey-status";
                }
                return;
            }
            console.error("Passkey registration error:", e);
            if (statusEl) {
                statusEl.textContent = "Error: " + e.message;
                statusEl.className = "passkey-status error";
            }
        }
    }

    /**
     * Authenticate with a passkey (discoverable / usernameless login).
     * Called from the login page.
     */
    async function loginWithPasskey(statusId) {
        var statusEl = document.getElementById(statusId);

        if (statusEl) {
            statusEl.textContent = "Starting authentication…";
            statusEl.className = "passkey-status";
        }

        try {
            // Step 1: Get assertion options from server.
            var beginResp = await fetch("/passkeys/login/begin");
            if (!beginResp.ok) {
                throw new Error("Server error: " + beginResp.status);
            }
            var options = await beginResp.json();

            // Decode binary fields.
            options.publicKey.challenge = base64urlToBuffer(options.publicKey.challenge);
            if (options.publicKey.allowCredentials) {
                options.publicKey.allowCredentials = options.publicKey.allowCredentials.map(function (c) {
                    c.id = base64urlToBuffer(c.id);
                    return c;
                });
            }

            if (statusEl) {
                statusEl.textContent = "Waiting for biometric…";
            }

            // Step 2: Prompt user for biometric.
            var assertion = await navigator.credentials.get(options);

            // Step 3: Encode response for server.
            var authResponse = assertion.response;
            var body = JSON.stringify({
                id: assertion.id,
                rawId: bufferToBase64url(assertion.rawId),
                type: assertion.type,
                response: {
                    authenticatorData: bufferToBase64url(authResponse.authenticatorData),
                    clientDataJSON: bufferToBase64url(authResponse.clientDataJSON),
                    signature: bufferToBase64url(authResponse.signature),
                    userHandle: authResponse.userHandle
                        ? bufferToBase64url(authResponse.userHandle)
                        : "",
                },
            });

            // Step 4: Send to server.
            var finishResp = await fetch("/passkeys/login/finish", {
                method: "POST",
                headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() },
                body: body,
            });

            if (!finishResp.ok) {
                var errBody = await finishResp.json().catch(function() { return {}; });
                throw new Error(errBody.error || "Authentication failed. Please try another sign-in method.");
            }

            var result = await finishResp.json();

            if (statusEl) {
                statusEl.textContent = "Success! Redirecting…";
                statusEl.className = "passkey-status success";
            }

            // Redirect to home.
            window.location.href = result.redirect || "/";
        } catch (e) {
            if (e.name === "NotAllowedError") {
                if (statusEl) {
                    statusEl.textContent = "Authentication cancelled.";
                    statusEl.className = "passkey-status";
                }
                return;
            }
            console.error("Passkey login error:", e);
            if (statusEl) {
                statusEl.textContent = "Error: " + e.message;
                statusEl.className = "passkey-status error";
            }
        }
    }

    // Check if WebAuthn is available in this browser.
    function isWebAuthnAvailable() {
        return window.PublicKeyCredential !== undefined &&
            typeof window.PublicKeyCredential === "function";
    }

    // Expose to global scope for onclick handlers.
    window.RepLogPasskeys = {
        register: registerPasskey,
        login: loginWithPasskey,
        isAvailable: isWebAuthnAvailable,
    };

    // On page load, show/hide passkey UI based on browser support.
    document.addEventListener("DOMContentLoaded", function () {
        var passkeyElements = document.querySelectorAll("[data-passkey]");
        if (!isWebAuthnAvailable()) {
            for (var i = 0; i < passkeyElements.length; i++) {
                passkeyElements[i].style.display = "none";
            }
        }
    });
})();
