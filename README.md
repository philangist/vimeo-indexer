[![Build Status](https://travis-ci.com/philangist/vimeo-indexer.svg?branch=master)](https://travis-ci.com/philangist/vimeo-indexer)  [![codecov](https://codecov.io/gh/philangist/vimeo-indexer/branch/master/graph/badge.svg)](https://codecov.io/gh/philangist/vimeo-indexer)


Vimeo Indexer
==
A multiplexer service that reads a stream of `(user,video)` pairs, fetches data from them from an Users and Videos service, and sents the combined result to a downstream Index service.

__Running it locally__

Requires a local Docker and Go installation

```bash
$ docker-compose up
# in another terminal window
$ export NUM_THREADS=5
$ export TIMEOUT=3
$ ./challenge-darwin input -c 100 --header | go run main.go
```

Tests:
```bash
$ go test -v
```

__Questions__  

*Was the question/problem clear? Did you feel like something was missing or not explained correctly?*
- The problem was well defined, but a few details were left out. The request is to "create a piece of software that collects data from two services and indexes that data through a third service as quickly and efficiently as possible", but a little more context would've been helpful. What does the typical load for our system look like (in other words, what value should i pass in for the -c flag to the `./challenge-linux input -c <LOAD>` command?). I'd also argue that the wording "as quickly and efficiently as possible" is ill-defined, and specific metrics around what success would look like might have been better. For example: We expect the service to index 95% of all incoming requests in under 5 seconds, to have a memory profile that fits this certain use case, minimize total bandwith usage, etc.
There were also some implementation details that were not defined ahead of time, for example the /users and /index endpoints return a gzip compressed response and that's not indicated in the problem definition.

*How much time did you spend on each part: understanding, designing, coding, testing?*

- Understanding & design:
After reading the assessment I wrote a small python script to parse 1000000 input values and see if there were any interesting patterns in the data that could be useful, but the output of the script seemed to be completely random (~10 minutes).
I then spent the next two days working on other projects and thinking over the possible ways to approach the problem. I knew I'd take a concurrent approach for solving it and since I hadn't used Go's concurrency model before I spent some time watching videos on Go concurrency patterns and came up with a single threaded version of the core solution.


- Coding:
I took about 3 days of working on and off to get to an implementation I thought was respectable. The core solution itself was written fairly early on and took 2-3 hours - https://github.com/philangist/vimeo-indexer/commit/c7e6b9b34063f5fadc16ff61c961aee11d417787 - everything after that has just been iteration and refinement. I did the difficulty of implementing the concurrent approach in Go however , and spent much of sunday fighting deadlocks and race conditions and trying to better understand the language. I had to write out a dead simple version of the core workflow (https://gist.github.com/philangist/d14bc3319e1eb4a9c9f983f6dd461fc0) and then I focused on addressing the entire problem space, and after aggresively simplify my core logic, refactoring, and testing


- Testing:
I was manually testing for the first few days but when I started to approach a stable solution I took about 5-6 hours over 2 nights to write tests
https://github.com/philangist/vimeo-indexer/commit/1fca7206888ec6b51cb8170569533f2ba49d7447
https://github.com/philangist/vimeo-indexer/commit/c9a82c90035074d4bd1eca115fd79ec98b2311f9

- It took a larger time investment than I initially expected, but I wanted to write a high-quality solution.


*Why did you choose the language to write your solution in?*
- Because Go is fun to write in! It is a language that was designed specifically for projects of this type; building small, performant networking services with out of the box concurrency and it has a great standard library backing it. The development cycle is also very fast (think tight inner loop) and the tooling around testing and performance was really helpful here


*What would you have done differently if you had more time or resources?*
- I would've really fleshed out what the concurrent implementation actually looks like before I started working on it, and I would've also made a better effort to REALLY understand Go's approach to concurrency
- Improve testing (concurrency. race condition here -> https://github.com/philangist/vimeo-indexer/blob/master/main_test.go#L121)

*Are there any bottlenecks with your solution? If so, what are they and
what can you do to fix them/minimize their impact?*
INSERT PERF PROFILE INFO HERE

- Yep. With an application like this you'd expect performance hotspots to primarily be i/o and the overhead costs of managing concurrency and from performance profiling that's definitely the case.

- the rate at which we can read the input stream from stdin is the first obvious bottleneck

- sending data over the wire to/from /users and /videos services is the next one

- last is sending data to /index service which can be overwhelmed by too many requests

- the error rate also significantly limits the systems overall throughput (if our architecture was a Digraph, the edges to /users and /index would have a much higher cost than any other. how does this affect max flow?)

- there's also the additional overhead of synchronizing multiple goroutines and goroutines blocking each other

__improvements__:
- reduce the amount of information we send back. the /users payload contains a lot of information that's seemingly not relevant to search queries, so do we really need to index their last ipaddress or which language they speak.

- consider a different encoding type. decompressing from gzip is the largest memory hog in our application. a cursory google search shows that deflate is about 40% faster than gzip

- use a rpc protocol like thrift or protobuf for communicating among internal services. the payload is typically much smaller and faster than a raw json request.

- make network requests to /users and /index async within PostIndexData

- there are also potential optimizations based on typical user behavior on the platform. do they often upload multiple videos at once? maybe it makes sense for the call to /index to be structures as
```json
{
    "user": {"user": "info"},
    "videos": [
      {"video": "info"},
      {"video": "info"},
      {"video": "info"}
    ]
}
```
- horizontal scaling of the number of servers running the /users, /videos, and /index services also. I was starting to see dropped messages on my local machine when I jacked the number of threads up to several hundred, because the other backend services could not keep up with the workload.

- optimizing memory usage. Escape analysis (we want as much data on the stack as possible. benefits of stack = improved cache hits due to locality of reference, we're not chasing pointers around the heap, reduces memory fragmentation), pre-allocate and reuse fixed size buffers/byte arrays in an object pool. take advantage of byte alignment when defining structs. use more performant libraries to replace json marshaling/unmarshaling, decompressing logic

- Send less data by questioning our assumptions. The most efficient code is that which isn't written, so it's worthwhile to ask are there any queries on the index that could be done in a more conventional datastore. a query like (all videos in the last month from region seattle that are at least 5 minutes with term "foo" in their title or body) can have the `range(video.date)`, `exact(video.region)`, `gte(video.length, 5 minutes)` filtering happen in a db and our index only needs to be aware of the values of title and body.

- cache the values of partially successful `user,video` pairs. so if we attempt to get a video id that fails, but it's user id passes, when we retry the indexing again later we'll only have to fetch once.

- improve the error rate of /users and /videos (biggest win) and /index as well

- from a ux perspective, since bottlenecks will always exists we can be smart about prioritizing so users are likely to find the results they want in a timely manner. encoding type information for the behavior that an event in the input stream matches and prioritizing based off that. t. ex: events that are of type create(video) should have a higher weight than update(video.frame_rate)

- the standard JSON library is also slow because it uses reflection

- it's important to keep in mind the optimization costs for many of these changes. it can make it more diffcult for humans to reason about application behavior, as well as make architectural changes due to changing requirements more expensive, so we should always try to maintain a balance between performance and maintainability. Donald Knuth: "Programmers waste enormous amounts of time thinking about, or worrying about, the speed of noncritical parts of their programs, and these attempts at efficiency actually have a strong negative impact when debugging and maintenance are considered. We should forget about small efficiencies, say about 97% of the time: premature optimization is the root of all evil. Yet we should not pass up our opportunities in that critical 3%."

*How would the system scale for a larger data set (1 billion+ or a never
ending stream) or to handle more complex queries or higher volume of
queries?*

 - scaling for a larger data set:
    the system starts to degrade in performance for larger data sets. memory usage will likely be the biggest issue, gc pauses will be larger and more frequent. partially because of the nature of the problem (forwarding large amounts of data from one group of services to another), partially because of implementation tradeoffs made at a smaller case. naively retrying every failed request infinitely might work fine for a data set of several hundred thousand or millions, but it quickly becomes too expensive  and because we infinitely retry all failed operations there can be network congestion and increased average latency for "legitimate" requests.

 - scaling for more complex/higher volume of queries:
    we're likely sending too much unneeded data to the data store anyway, the size of documents might. I'd argue that as query volume increases we should really focus on doing a few things really well so we can have stronger guarantees about our system's behavior.

 - dealing with failure of an instance:

 - dealing with failure of our entire cluster:

 - we'd likely want to move to a persistent message bus (like kafka) for data integrity guarantees (we want every message to be processed at least once) instead of doing it all in memory with channels

 - horizontal scaling of the service, spin up several instances and hide them behind a load balance

*Anything else you want to share about your solution or the problem :)*
- This was a really fun project and i put a lot of effort into it. i hope that's reflected in the quality of the work
