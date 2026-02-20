/**
 * RepLog — declarative UI utilities.
 *
 * Replaces inline onclick handlers with data-attribute–driven behavior.
 * All handlers are attached via event delegation on document.body so they
 * work with htmx-swapped content without rebinding.
 *
 * Supported data attributes:
 *
 *   data-toggle="<selector>"        Toggle hidden on the target element.
 *   data-show="<selector>"          Remove hidden from target, add hidden to self.
 *   data-hide="<selector>"          Add hidden to target, remove hidden from trigger.
 *   data-toggle-edit="<scope>"      Show/hide edit mode within a scoped container.
 *       The scope is a CSS selector for the nearest ancestor. Within it:
 *         .edit-toggle-display  — hidden when editing
 *         .edit-toggle-form     — shown when editing
 *         .edit-toggle-trigger  — the trigger button itself (hidden when editing)
 *   data-cancel-edit="<scope>"      Cancel inline editing and restore display mode.
 *   data-toggle-row="<id>"          Toggle visibility of a table row by id.
 *   data-hide-row                   Hides the closest .edit-row (cancel in table).
 *   data-copy-from="<selector>"     Copy value from an input to clipboard.
 *   data-select-on-focus            Select the input contents on focus.
 *   data-print                      Trigger window.print().
 *   data-new-athlete-toggle         Toggle new-athlete-fields based on select value.
 */
(function () {
    "use strict";

    // ---- Helper: find closest ancestor matching selector ----
    function up(el, sel) {
        return el.closest(sel);
    }

    // ---- Copy to clipboard with feedback ----
    function copyToClipboard(btn, sourceSelector) {
        var source = sourceSelector
            ? document.querySelector(sourceSelector) || btn.previousElementSibling
            : btn.previousElementSibling;
        if (!source) return;
        var text = source.value || source.textContent;
        navigator.clipboard.writeText(text).then(function () {
            var orig = btn.textContent;
            btn.textContent = "Copied!";
            setTimeout(function () { btn.textContent = orig; }, 2000);
        });
    }

    // ---- Main click delegation ----
    document.addEventListener("click", function (e) {
        var btn = e.target.closest("[data-toggle]");
        if (btn) {
            var target = document.querySelector(btn.getAttribute("data-toggle"));
            if (target) target.hidden = !target.hidden;
            return;
        }

        btn = e.target.closest("[data-show]");
        if (btn) {
            var target = document.querySelector(btn.getAttribute("data-show"));
            if (target) { target.hidden = false; btn.hidden = true; }
            return;
        }

        btn = e.target.closest("[data-hide]");
        if (btn) {
            var hideSel = btn.getAttribute("data-hide");
            var showSel = btn.getAttribute("data-show-on-hide");
            var target = document.querySelector(hideSel);
            if (target) target.hidden = true;
            if (showSel) {
                var showEl = document.querySelector(showSel);
                if (showEl) showEl.hidden = false;
            }
            return;
        }

        btn = e.target.closest("[data-toggle-edit]");
        if (btn) {
            var scope = up(btn, btn.getAttribute("data-toggle-edit"));
            if (scope) {
                var display = scope.querySelector(".edit-toggle-display");
                var form = scope.querySelector(".edit-toggle-form");
                var trigger = scope.querySelector(".edit-toggle-trigger");
                if (display) display.hidden = true;
                if (form) {
                    form.hidden = false;
                    var ta = form.querySelector("textarea, input[type='text']");
                    if (ta) ta.focus();
                }
                if (trigger) trigger.hidden = true;
            }
            return;
        }

        btn = e.target.closest("[data-cancel-edit]");
        if (btn) {
            var scope = up(btn, btn.getAttribute("data-cancel-edit"));
            if (scope) {
                var display = scope.querySelector(".edit-toggle-display");
                var form = scope.querySelector(".edit-toggle-form");
                var trigger = scope.querySelector(".edit-toggle-trigger");
                if (form) form.hidden = true;
                if (display) display.hidden = false;
                if (trigger) trigger.hidden = false;
            }
            return;
        }

        btn = e.target.closest("[data-toggle-row]");
        if (btn) {
            var row = document.getElementById(btn.getAttribute("data-toggle-row"));
            if (row) row.hidden = !row.hidden;
            return;
        }

        btn = e.target.closest("[data-hide-row]");
        if (btn) {
            var row = up(btn, ".edit-row");
            if (row) row.hidden = true;
            return;
        }

        btn = e.target.closest("[data-copy-from]");
        if (btn) {
            copyToClipboard(btn, btn.getAttribute("data-copy-from"));
            return;
        }

        btn = e.target.closest("[data-copy]");
        if (btn) {
            copyToClipboard(btn, null);
            return;
        }

        btn = e.target.closest("[data-print]");
        if (btn) {
            window.print();
            return;
        }

        // Passkey actions — delegate to RepLogPasskeys if available.
        btn = e.target.closest("[data-action='passkey-login']");
        if (btn) {
            if (window.RepLogPasskeys) RepLogPasskeys.login("passkey-login-status");
            return;
        }

        btn = e.target.closest("[data-action='passkey-register']");
        if (btn) {
            if (window.RepLogPasskeys) RepLogPasskeys.register("passkey-label", "passkey-register-status");
            return;
        }
    });

    // ---- Focus delegation for select-on-focus inputs ----
    document.addEventListener("focusin", function (e) {
        if (e.target.hasAttribute("data-select-on-focus")) {
            e.target.select();
        }
    });

    // ---- Change delegation for new-athlete-toggle and auto-submit ----
    document.addEventListener("change", function (e) {
        if (e.target.hasAttribute("data-new-athlete-toggle")) {
            var fields = document.getElementById("new-athlete-fields");
            if (fields) {
                fields.hidden = e.target.value !== "__new__";
            }
        }
        if (e.target.hasAttribute("data-auto-submit")) {
            var form = e.target.closest("form");
            if (form) form.submit();
        }
    });

    // ---- Submit delegation for long-running forms ----
    document.addEventListener("submit", function (e) {
        // Generate form: show busy indicator when submitting.
        var form = e.target.closest("[data-generate-submit]");
        if (form) {
            var btn = form.querySelector("#generate-btn");
            var indicator = form.querySelector("#generate-indicator");
            if (btn) {
                btn.setAttribute("aria-busy", "true");
                btn.textContent = "Generating\u2026";
            }
            if (indicator) {
                indicator.style.display = "block";
            }
        }
        // Confirm-submit: require confirmation before submitting.
        // Can be placed on a form or on a specific submit button within a form.
        var confirmForm = e.target.closest("[data-confirm-submit]");
        if (!confirmForm && e.submitter) {
            confirmForm = e.submitter.closest("[data-confirm-submit]");
        }
        if (confirmForm) {
            var msg = confirmForm.getAttribute("data-confirm-submit");
            if (msg && !window.confirm(msg)) {
                e.preventDefault();
            }
        }
    });
})();
