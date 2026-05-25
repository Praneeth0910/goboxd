# Architecture Decision Records (ADRs)
This document contains the Architecture Decision Records (ADRs) for the AI project. ADRs are a way to capture important architectural decisions made during the development of the project, along with their context and consequences.
## Choice of chi over net/http, gin and echo
 **Context**: The project requires a lightweight and efficient HTTP router that can handle high concurrency and provide good performance. I evaluated several popular Go web frameworks, including net/http, gin, echo, and chi. 
**Options considered**:
1. net/http: The standard library for HTTP in Go, which is simple and efficient but lacks some features provided by third-party frameworks.
2. gin: A popular web framework that offers a rich set of features, including middleware support, routing, and performance optimizations. However, it can be heavier than necessary for our use case.
3. echo: Another popular web framework that provides a similar feature set to gin, but with a different design philosophy. It is also heavier than necessary for our use case.
4. chi: A lightweight and modular HTTP router that focuses on simplicity and performance. It provides a minimalistic API and allows for easy composition of middleware, making it a good fit for our project.
**Decision**: I chose chi as the HTTP router for the project due to its lightweight nature, modular design, and good performance. It provides the necessary features for our use case without the overhead of more feature-rich frameworks like gin and echo. Additionally, chi's middleware composition allows for greater flexibility in handling requests and responses, which is beneficial for our project's requirements.
**Consequences**: I need to implement some features ourselves that are provided out-of-the-box by gin and echo, such as request validation and error handling. Overall, the decision to use chi aligns well with my project's goals of simplicity and efficiency while still providing the necessary functionality for our HTTP routing needs.

---

## Multi-Stage Docker Builds for Image Size and Reproducibility

**Context**: Need efficient Docker builds that minimize final image size while keeping all build tools and compiled binaries reproducible across environments.

**Options considered**:
1. Single-stage build: All tools in final image (~2GB bloat)
2. External builds: Run builds outside Docker, COPY binaries (version mismatch risk)
3. Multi-stage builds: Separate stages for nsjail → Go app → final runtime

**Decision**: Use 3-stage builds: nsjail-builder (compile from source) → go-builder (compile Go) → toolchains (runtime only).

**Consequences**: 
-  Smaller final image (~500MB vs 2GB)
-  Reproducible: nsjail pinned via git submodule tag 3.4
-  Fast iteration: Docker layer caching across stages
-  Longer first build (~5-10 min with full compilation)
