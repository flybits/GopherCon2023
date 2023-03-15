# GopherCon2023
Repo containing proposal and if accepted, code for the GopherCon2023 talk

https://www.papercall.io/gophercon-2023




Title
-----

Building resiliency and recovery in Go (gRPC) streaming applications

Talk Format
-----------

Tutorial session - 45 minutes

Audience Level
--------------

> All (or maybe intermediate) -- open to suggestions

Elevator Pitch
--------------

You have 300 characters to sell your talk. This is known as the "elevator pitch". Make it as exciting and enticing as possible.

> Let’s build a Kubernetes-deployed Go application that performs gRPC streaming between services and make it resilient to interruptions caused by application/network errors. Furthermore, let's make it auto-recover from forceful terminations where graceful interruption is not possible (SIGKILL signal)!

Description
-----------

This field supports Markdown. The description will be seen by reviewers during the CFP process and may eventually be seen by the attendees of the event.

You should make the description of your talk as compelling and exciting as possible. Remember, you're selling both the organizers of the events to select your talk, as well as trying to convince attendees your talk is the one they should see.

> This tutorial is for Gophers who are passionate about the reliability and integrity of their application in an operationally unpredictable environment, or even if the operation configuration lets them down! In more detail, using demos we’ll see what nasty things can happen to a naive application that performs gRPC streaming (or other long running tasks) on a Kubernetes cluster, and what we can do about each case. These include building resiliency by gracefully interrupting goroutines that perform long running tasks, mechanism for resuming an interrupted task from the point of interruption, and recovery mechanisms where graceful interruption is not possible (and there are some!). 

Notes
-----

This field supports Markdown. Notes will only be seen by reviewers during the CFP process. This is where you should explain things such as technical requirements, why you're the best person to speak on this subject, etc...

please see [this](https://github.com/flybits/GopherCon2023/blob/main/Notes.md).
