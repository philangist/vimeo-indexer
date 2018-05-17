[![Build Status](https://travis-ci.com/philangist/vimeo-indexer.svg?branch=master)](https://travis-ci.com/philangist/vimeo-indexer)  [![codecov](https://codecov.io/gh/philangist/vimeo-indexer/branch/master/graph/badge.svg)](https://codecov.io/gh/philangist/vimeo-indexer)


Vimeo Indexer
==
A multiplexer service that reads a stream of `(user,video)` pairs, fetches data about them from an Users and Videos service, and sends the combined results to a downstream Index service.

![](https://thumbs.gfycat.com/SnarlingFocusedKoalabear-size_restricted.gif)

__Running it locally__

Requires a local Docker and Go installation

```bash
$ go get github.com/philangist/vimeo-indexer
$ cd $GOPATH/src/github.com/philangist/vimeo-indexer
# Spin up 1 local instance for each of the Users, Videos, and Index services
$ docker-compose up
Creating network "vimeo-indexer_default" with the default driver
Creating vimeo-indexer_index_1  ... done
Creating vimeo-indexer_videos_1 ... done
Creating vimeo-indexer_users_1  ... done
Attaching to vimeo-indexer_index_1, vimeo-indexer_users_1, vimeo-indexer_videos_1
# Open a new terminal window and run the multiplexer
$ export NUM_THREADS=5
$ export TIMEOUT=3
$ ./challenge-linux input -c 100 --header | go run main.go
Running on 5 threads with a timeout of 3 seconds
Elapsed time:  9.676345979s
```

Tests:
```bash
$ go test -v
```

__Design Rationale__

*Was the question/problem clear? Did you feel like something was missing or not explained correctly?*
- The question was well defined, but some important details about constraints were left out. The request was to "create a piece of software that collects data from two services and indexes that data through a third service as quickly and efficiently as possible" but without knowledge of what the typical load for our system is the word "efficient" is ambiguous. Specific metrics of success would've helped with establishing a scope of work for the problem. For example, a problem specification like "the service should index 95% of all incoming requests in under 3 seconds, have a memory profile that is performant while running on an AWS t2.medium EC2 instance, and process 10000 user and video ids in 2 minutes".

- There was also a quirk of the Users and Videos services API that was not mentioned ahead of time, specifically that the /users and /index endpoints return a gzip compressed response.

*How much time did you spend on each part: understanding, designing, coding, testing?*

The  assessment took a much higher time investment than I initially expected, but I wanted to write a high-quality solution I'd be proud of.

Understanding & design:
- After reading the assessment I wrote a small python script to parse 1000000 input values and see if there were any patterns in the data that could be useful, but the output of the script seemed to be completely random. This took  less than 10 minutes.
- I then spent the next two days working on other projects and thinking over the possible ways to approach the problem. I knew I'd take a concurrent approach and since I hadn't used Go's concurrency model before I watched a few videos on Go concurrency patterns and drew a design for a single threaded implementation.

Coding:
- It took 3 days of working a few hours each day get to an implementation I thought was respectable. The core solution itself was written within the first session and took 2-3 hours - https://github.com/philangist/vimeo-indexer/commit/c7e6b9b34063f5fadc16ff61c961aee11d417787 - everything after that was focused on iteration and refinement. I did have difficulty implementing the concurrent solution, and spent most of Sunday fighting deadlocks and race conditions while trying to better understand Go's concurrency model. I had to write a dead simple version of the pipeline (https://gist.github.com/philangist/d14bc3319e1eb4a9c9f983f6dd461fc0) to clarify my thinking. After that I focused on addressing the entire problem space and edge cases, and then on aggresively simplify the core logic, refactoring, and testing.

- Testing:
I was manually testing for the first few days but when I started to see a solid implementation emerge I took about 5-6 hours over 2 nights to write tests
https://github.com/philangist/vimeo-indexer/commit/1fca7206888ec6b51cb8170569533f2ba49d7447
https://github.com/philangist/vimeo-indexer/commit/c9a82c90035074d4bd1eca115fd79ec98b2311f9

*Why did you choose the language to write your solution in?*
- I chose Go because it is a language that was designed specifically for projects of this type; building small, performant networking services with strong out of the box concurrency primitives. It also doesn't hurt that Go has a great standard library built in. The development feedback cycle is also very fast, and the tooling around testing and performance was really helpful. Last but not least, Go is really fun to write!

*What would you have done differently if you had more time or resources?*
- I would've fleshed out what the concurrent implementation actually looks like before I started working on it, and I would've also made a better effort to REALLY understand Go's approach to concurrency.
- I'd also work to improve the tests. There were two specific issues I had - I'm not clear what the idiomatic way to test channels is (https://github.com/philangist/vimeo-indexer/blob/master/main_test.go#L121) and the end-to-end test I wrote for the IndexService is very verbose. (https://github.com/philangist/vimeo-indexer/blob/master/main_test.go#L365).
- Split out the main.go package smaller modules/packages.
- Increase test coverage.

*Are there any bottlenecks with your solution? If so, what are they and
what can you do to fix them/minimize their impact?*

- Yep. With an application like this you'd expect performance bottlenecks around I/O and management of concurrent threads of execution (goroutines). The performance profile validates this assumption [as can be seen here](cpu_usage.gif).

- The rate at which we can read the input stream from stdin is the first obvious bottleneck, After that is communicating over the wire to the Users and Videos services, and last is sending data to the Index service. This is made worse by the fact that the Index service can be overwhelmed by too many requests and starts dropping messages.

- The builtin error rate also significantly limits the system's throughput. If our architecture was represented as a weighted digraph, the maximum flow is bounded by the weight of the outgoing edges of the Users and Videos service vertices.

- There's also the additional overhead of goroutines blocking while waiting to read off the input stream or request/send data downstream.

Potential Solutions:
- Reduce the amount of information sent back from the Users service. The payload contains a lot of information that's probably not relevant for search queries, so it's worth questioning if we need to index a user's last ipaddress or which language she speaks.

- Consider a different content/encoding type. Decompressing from gzip is the [largest memory hog in the application](heap_usage.gif) and a cursory google search shows that deflate is about 40% faster than gzip.

- Use a RPC protocol like Apache's Thrift or Google's Procol Buffers for communication among internal services. Their payloads are typically much smaller and delivery faster than a raw JSON request. We also don't have to pay the cost of Go's builtin `"encoding/json"` package which uses reflection for unmarshalling.

- Make network requests to Users and Videos services async within `IndexService.PostIndex`.

- There's also a whole class of optimizations that can be made depending on the usage patterns observed on the platform. For example, if users typically upload multiple videos at once it makes a lot more sense for the Index schema to be structured like
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
- We can also horizontally scale out the number of servers running the Users, Videos, and Index services. I was starting to see dropped messages on my local machine when I jacked the number of goroutines POSTing to Index up to several hundred, because it could not keep up with the workload.

- Optimize memory usage. Reason through the escaping behavior of variables in the codebase (or use the builtin tooling around that) and try to allocate as much data on the call stack as possible. This will increase the CPU cache hit rate since we're not chasing pointers around the heap, reduce memory fragmentation, and lower the heap's size. All of this will also help lighten the load on the GC leading to shorter and less frequent pauses.

- We can also re-allocate and reuse fixed size buffers/byte arrays to reduce copying of data/prevent the heap from growing. Go provides `sync.Pool` as a built-in object pool manager, so the cost of implementation should be very low.

- Take advantage of byte alignment when defining structs to minimize the size of our data structures.

- Use more performant external libraries to replace JSON marshalling and umarshalling, request body decompression, etc.

- Questioning our assumptions to reduce the scope of work. The most efficient code is code that isn't written in the first place, so it's worthwhile to ask if there is any logic on the search platform that can be ripped out or that should live elsewhere. For example, we can try fo find existing queries on the Index that can be done in a more conventional datastore. A query like 'give me all videos in the last month from region seattle that are at least 5 minutes long with term "foo" in their title or body' can have the `range(video.date)`, `exact(video.region)`, `gte(video.length, 5 minutes)` filtering happen in a RDBMS and Index only needs to be aware of the values of `title` and `body`.

- Cache the values of partially successful `user,video` pairs. So if we attempt to get a video that fails, but it's user is successfuly returned, when we see the pair again we'll only have to make one request.

- Lower the error rate of the Users and Videos services (biggest win) and Index as well.

- From a UX perspective, since we know that bottlenecks will always exists, we can be smart about ordering requests so that the results user care about are indexed first. Consider a scenario where we add a `type` attribute to every `user,video` pair and schedule outbound requests using `type` as the sort key. We can then enforce conditions like: events that are of `type=create(video)` should have a be indexed sooner than `type=update(video.frame_rate)`.

- It's important to keep in mind the additional complexity costs for some of these optimizations. They can introduce subtle bugs and also make it more diffcult for humans to reason about application behavior, onboard new developers, make architectural changes, etc. We should always maintain a careful balance between performant and maintainable code. Donald Knuth: "Programmers waste enormous amounts of time thinking about, or worrying about, the speed of noncritical parts of their programs, and these attempts at efficiency actually have a strong negative impact when debugging and maintenance are considered. We should forget about small efficiencies, say about 97% of the time: premature optimization is the root of all evil. Yet we should not pass up our opportunities in that critical 3%."

*How would the system scale for a larger data set (1 billion+ or a never
ending stream) or to handle more complex queries or higher volume of
queries?*

A lot of the failure conditions that emerge as scale increases were identified and addressed in the previous section, but it's worthwhile to call out the following potential issues:
 - Scaling for a larger data set:
    The system **will** start to degrade in performance for larger data sets. Memory usage will likely be the biggest issue, and gc pauses will become larger and more frequent. This is partially due to the nature of the problem (forwarding large amounts of data from one group of services to another), and partially because of implementation tradeoffs that made sense at a smaller scale. Naively retrying every failed request indefinitely might work fine for a data set of several hundred thousand or a few million, but it quickly becomes too expensive at 1 billion + requests and will increase both network congestion and average latency for "legitimate" requests.

 - Scaling for more complex/higher volume of queries:
    Introducing more complex queries necessarily increases the total number of possible failure modes, and also increases the variance of request latency - which means out P99 latency values will probably be much higher.
    I'd argue that as query volume increases we should really focus on reducing document size (only index the smallest subset of attributes we can get away with) and query types. It's a conservative approach, but doing a few things really well means we can have stronger guarantees about our system's behavior.

 - At the scale of billions (or an infinite stream) it also becomes important to introduce layers of redundancy for handling complex failure modes. We're likely going to have to spin up several instances of the multiplexer service and hide them behind a load balancer. We'd also likely want to move to a persistent message bus like Kafka for data integrity guarantees (we want every message to be processed at least once) instead of trying to do it all in memory with Go channels.

*Anything else you want to share about your solution or the problem :)*
- This was a really fun project and I put a lot of effort into it. Hopefully that's reflected in the quality of my work.
