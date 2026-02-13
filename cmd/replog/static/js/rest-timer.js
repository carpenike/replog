// rest-timer.js — Countdown rest timer between sets.
// Activated by ?timer=SECONDS query parameter on workout detail page.
// Persists across htmx navigations using the timer container element.
(function () {
    "use strict";

    var timerEl = document.getElementById("rest-timer");
    if (!timerEl) return;

    var params = new URLSearchParams(window.location.search);
    var totalSeconds = parseInt(params.get("timer"), 10);
    if (!totalSeconds || totalSeconds <= 0) {
        timerEl.style.display = "none";
        return;
    }

    // Clean the URL so a browser refresh doesn't restart the timer.
    if (window.history.replaceState) {
        var cleanURL = window.location.pathname + window.location.hash;
        window.history.replaceState(null, "", cleanURL);
    }

    var remaining = totalSeconds;
    var intervalID = null;
    var running = true;

    var display = timerEl.querySelector(".timer-display");
    var progress = timerEl.querySelector(".timer-progress");
    var toggleBtn = timerEl.querySelector(".timer-toggle");
    var resetBtn = timerEl.querySelector(".timer-reset");
    var dismissBtn = timerEl.querySelector(".timer-dismiss");

    timerEl.style.display = "";

    function formatTime(sec) {
        var m = Math.floor(sec / 60);
        var s = sec % 60;
        return m + ":" + (s < 10 ? "0" : "") + s;
    }

    function render() {
        display.textContent = formatTime(remaining);
        if (progress) {
            var pct = totalSeconds > 0 ? ((totalSeconds - remaining) / totalSeconds) * 100 : 100;
            progress.style.width = pct + "%";
        }
        if (remaining <= 0) {
            timerEl.classList.add("timer-done");
            display.textContent = "REST COMPLETE";
        } else {
            timerEl.classList.remove("timer-done");
        }
    }

    function tick() {
        if (remaining > 0) {
            remaining--;
            render();
            if (remaining === 0) {
                stop();
                beep();
            }
        }
    }

    function start() {
        if (intervalID) return;
        running = true;
        intervalID = setInterval(tick, 1000);
        if (toggleBtn) toggleBtn.textContent = "Pause";
    }

    function stop() {
        if (intervalID) {
            clearInterval(intervalID);
            intervalID = null;
        }
        running = false;
        if (toggleBtn) toggleBtn.textContent = "Resume";
    }

    function reset() {
        stop();
        remaining = totalSeconds;
        timerEl.classList.remove("timer-done");
        render();
        start();
    }

    function dismiss() {
        stop();
        timerEl.style.display = "none";
    }

    // Simple beep using Web Audio API — no external audio file needed.
    function beep() {
        try {
            var ctx = new (window.AudioContext || window.webkitAudioContext)();
            // Play two short tones.
            [0, 0.2].forEach(function (offset) {
                var osc = ctx.createOscillator();
                var gain = ctx.createGain();
                osc.connect(gain);
                gain.connect(ctx.destination);
                osc.frequency.value = 880;
                osc.type = "sine";
                gain.gain.value = 0.3;
                osc.start(ctx.currentTime + offset);
                osc.stop(ctx.currentTime + offset + 0.15);
            });
        } catch (e) {
            // Audio not available — fail silently.
        }
    }

    if (toggleBtn) {
        toggleBtn.addEventListener("click", function () {
            if (running) { stop(); } else { start(); }
        });
    }
    if (resetBtn) {
        resetBtn.addEventListener("click", reset);
    }
    if (dismissBtn) {
        dismissBtn.addEventListener("click", dismiss);
    }

    render();
    start();
})();
