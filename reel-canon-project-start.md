# Reel Canon

> **Financing clarification (2026-10-06):** Reel Canon is an ambitious hobby project. Cover running costs first; around EUR 200/month would be a welcome later result, without a short-term profitability requirement. Voluntary support, ads/one-time ad removal and future streaming Pro are options, not settled prices or enabled features. See the [hobby financing concept](docs/hobby-financing-concept.md), which takes precedence over earlier monetization assumptions below.

> **Current MVP decisions (2026-10-05):** Start with IMDb Top 250, build small crawling modules per Canon with configurable refresh intervals (Top 250: every 1–3 months later), enrich movies through TMDb, and store progress locally without end-user authentication or cloud sync. Canon administration runs separately; the proposed simpler implementation is a protected Go server with HTML templates, sharing the central catalog and backend domain logic. Live crawling is not required now. See [MVP decisions](docs/mvp-decisions.md). These decisions take precedence over conflicting initial-launch, authentication, and persistence statements below; the remainder describes the broader product vision.

> A curated film journey for people who want to discover, collect, and complete the essential films of cinema.

## 1. Project Overview

**Reel Canon** is a film-tracking companion focused on curated movie collections and long-term completion goals.

The initial inspiration is the book **“1001 Movies You Must See Before You Die”**, but Reel Canon is intentionally designed as a broader platform rather than a companion for only one list.

Users can:

- explore curated film collections (“Canons”)
- mark films as watched
- track completion progress
- browse films by decade, genre, director, year, rating, and other metadata
- search across all included films
- save progress with a user account
- visualize progress through a collectible, sticker-album-inspired interface
- unlock achievements and completion goals
- optionally use advanced streaming availability features
- later unlock additional curated Canons individually or through Reel Canon Pro

The long-term goal is to turn film discovery and completion into an elegant, collectible experience rather than a conventional checklist.

---

# 2. Brand

## Name

**Reel Canon**

## Positioning

Reel Canon should feel like a premium editorial film product rather than a generic movie database.

The visual direction is inspired by:

- The New York Times / NYT Magazine
- A24
- premium film magazines
- film archives
- coffee-table cinema books
- tasteful physical collector editions

The product should feel:

- editorial
- cinematic
- mature
- calm
- collectible
- high-quality
- modern
- not overly “app-like”
- not visually similar to Netflix or IMDb

Material Design concepts may be used for interaction patterns, but the final visual language should remain highly customized.

---

# 3. Core Product Concept

The product is based around **curated film collections**, internally called **Canons**.

Examples:

- 1001 Movies You Must See Before You Die
- IMDb Top 250
- Sight & Sound 100
- Academy Award Best Picture Winners
- Palme d’Or Winners
- Sci-Fi Essentials
- Horror Essentials
- 1980s Essentials
- Director-specific collections
- future Reel Canon editorial collections

Each Canon has:

- a title
- description
- cover / visual identity
- list of films
- completion percentage
- completed-film count
- achievements
- optional premium status

A film may belong to multiple Canons.

A user marks a film as watched **once**.

That state automatically applies to every Canon containing the film.

Example:

If *The Godfather* belongs to:

- 1001 Movies
- IMDb Top 250
- Best Picture Winners

then marking it as watched updates progress in all three Canons.

---

# 4. Initial Canon

The first and primary Canon for launch:

## 1001 Movies

This serves as:

- the initial product focus
- the default free collection
- the main onboarding experience
- the first large data set
- the first use case for progress tracking and album completion

The architecture must **not** hard-code the application around the 1001 Movies list.

Instead, 1001 Movies should simply be the first Canon in a general collection system.

---

# 5. MVP

The MVP is a responsive web application for desktop and mobile.

Native apps are explicitly **not** required for the first version.

## MVP Goals

The MVP should already feel like a complete product.

It should allow a user to:

1. browse the full initial Canon
2. search films
3. filter films
4. open film detail pages
5. mark films as watched or unwatched
6. optionally maintain a watchlist
7. see total progress
8. browse progress by decade
9. use the sticker-album-style collection view
10. log in and sync progress across devices

## MVP Features

### Film Library

Each film should support metadata such as:

- title
- original title
- release year
- decade
- runtime
- genre
- director
- country
- IMDb ID
- TMDb ID
- poster
- description / overview
- IMDb rating
- associated Canons

### Search

Search by:

- title
- original title
- director

Potential later expansion:

- cast
- country
- keywords

### Filters

MVP filtering should support:

- watched / unwatched
- year
- decade
- genre
- director
- IMDb rating

Later:

- country
- runtime
- streaming provider
- Canon
- personal rating
- achievement-related filters

### Sorting

Useful sorting options:

- title
- year
- IMDb rating
- recently watched
- oldest unseen
- newest unseen

### Film Detail Page

A film page should show:

- poster
- title
- original title
- year
- runtime
- director
- genre
- country
- IMDb rating
- short overview
- Canons containing the film
- watched state
- optional watchlist state

Potential future information:

- personal rating
- watched date
- personal notes
- streaming availability
- edition history
- related achievements

---

# 6. Progress & Collection Experience

The progress page should not feel like a conventional statistics dashboard.

Its key visual concept is a **premium sticker album / collector album**.

## Sticker Album Concept

Films are grouped by decade.

Examples:

- 1920s
- 1930s
- 1940s
- 1950s
- …
- 2020s

The user can browse or “flip” through decades.

### Film States

#### Unwatched

The poster appears:

- black and white
- desaturated
- optionally slightly blurred
- subdued
- visually incomplete

#### Watched

The poster becomes:

- full color
- sharp
- visually prominent
- “collected”

The result should make the collection become more colorful and complete as the user watches more films.

This is one of Reel Canon’s defining visual mechanics.

## Interaction

Desktop:

- previous / next decade navigation
- keyboard navigation
- trackpad support where appropriate
- decade selector

Mobile:

- horizontal swipe between decades
- compact decade selector
- responsive poster grid

## Decade Header

Example:

```text
1970s

63 / 89 films collected
71% complete
```

Optional supporting data:

- top directors
- dominant genres
- remaining films
- closest achievement
- time spent watching films from the decade

## Completion State

When all films in a decade have been watched:

```text
1970s — COMPLETE
89 / 89
Completed October 2027
```

The presentation should feel like:

- an archival stamp
- premium print
- a collector badge

It should **not** resemble a childish trophy system.

---

# 7. User Film States

The minimum state is:

- watched
- unwatched

Recommended expanded state model:

- unwatched
- watchlist
- watched
- favorite

Potential fields:

```text
watched
watched_at
watchlist
favorite
personal_rating
notes
```

---

# 8. Achievements & Gamification

Achievements are a planned later feature and are primarily intended as part of Reel Canon Pro.

The purpose is to encourage exploration without making the product feel like a game.

## Achievement Categories

### Completion

Examples:

- 100 films watched
- 250 films watched
- 500 films watched
- 750 films watched
- final 100 remaining
- complete 1001 Movies
- complete a Canon

### Decades

Examples:

- complete the 1950s
- complete the 1970s
- complete the 1990s
- watch films from every represented decade

### Genres

Examples:

- complete all Film Noir films
- complete all Science Fiction films
- complete all Horror films
- watch films from ten different genres

### Directors

Examples:

- watch every Hitchcock film in a Canon
- complete Kubrick
- complete Kurosawa
- complete Bergman
- complete a director-specific Canon

### Film History

Examples:

- watch 10 silent films
- watch films from 20 countries
- complete a certain movement or era
- watch films from multiple continents

### Activity-Based

Potential later achievements:

- three films in one weekend
- monthly completion goals
- seasonal challenges
- watch films from five genres in one month

These should be used carefully to avoid encouraging unhealthy viewing behavior.

## Presentation

Achievements should use the editorial visual language.

Example:

```text
THE 1970s
Completed

89 / 89 films
October 2027
```

Avoid:

- cartoon badges
- excessive confetti
- bright gaming colors
- noisy trophy interfaces

---

# 9. “Your Journey” Dashboard

A future progress dashboard may combine statistics and achievements.

Example:

```text
YOUR JOURNEY

421 / 1001

42% complete

12 achievements
3 decades completed
4 genres completed
```

## Next Milestones

Examples:

```text
7 films left to complete the 1970s

2 films left for Hitchcock Complete

1 Japanese film left to unlock Tokyo Stories
```

This can create motivation without dominating the product.

---

# 10. Streaming Availability

Streaming availability is an advanced feature and a strong candidate for Reel Canon Pro.

## Basic Streaming Information

A film may show where it is currently available.

Examples:

- Netflix
- Prime Video
- MUBI
- Disney+
- Apple TV
- ARTE
- rental
- purchase

Availability is country-dependent.

The data model should therefore include region / country.

## Streaming Filters

Users should eventually be able to filter by:

- available on my services
- subscription streaming
- rent
- purchase
- free / ad-supported
- provider

Example:

```text
Unwatched
+
Available on my services
+
Runtime < 120 min
+
IMDb > 7.5
```

This enables a strong use case:

> What should I watch tonight?

## Strong Pro Feature

Example:

```text
127 of your unwatched films are currently available
on your streaming services.
```

Provider breakdown:

```text
MUBI          42 unseen
Prime Video   31 unseen
Netflix       18 unseen
ARTE           9 unseen
```

## Streaming + Completion

A particularly valuable feature:

```text
7 films currently available on your streaming services
would complete your 1980s collection.
```

This combines:

- tracking
- discovery
- streaming data
- completion
- gamification

and should be treated as a premium differentiator.

## Architecture Requirement

Streaming integrations must be provider-independent.

Example backend abstraction:

```text
StreamingProvider

getAvailability(movieId, country)
```

Possible implementations:

```text
TmdbStreamingProvider
JustWatchProvider
WatchmodeProvider
```

The application should not tightly couple itself to one external streaming data vendor.

Licensing and commercial-use restrictions must be reviewed before launch.

---

# 11. Authentication

Users should be able to use Reel Canon anonymously for browsing.

Saving progress across devices requires an account.

## Supported Authentication

Planned:

- Google
- Apple

Potential later options:

- email magic link
- passkeys

User accounts primarily exist for:

- progress sync
- watchlist
- ratings
- achievements
- purchased Canons
- Pro membership
- user settings

---

# 12. Monetization

Reel Canon should support a combination of free access, Pro features, and potentially one-time Canon unlocks.

## Free

Possible free offering:

- 1001 Movies Canon
- film browsing
- search
- basic filters
- watched tracking
- basic progress
- sticker album
- basic completion milestones

The free product must remain useful on its own.

## Reel Canon Pro

Possible Pro features:

- all premium Canons
- streaming availability
- streaming filters
- advanced statistics
- full achievement system
- monthly / seasonal challenges
- advanced recommendation filters
- streaming + completion suggestions
- additional personalization
- future custom Canons

Potential examples:

```text
Reel Canon Pro
All Canons. Streaming. Achievements. Advanced Stats.
```

Pricing is intentionally undecided at this stage.

## Individual Canon Unlocks

Premium Canons may also be unlockable individually.

Example:

```text
Unlock this Canon — €0.99
```

This gives users an alternative to a subscription.

Potential model:

```text
Free
    1001 Movies

€0.99 one-time
    individual premium Canon

Reel Canon Pro
    all premium Canons
    + streaming
    + achievements
    + advanced features
```

Pricing such as **€0.99 per Canon** is currently conceptual and should be validated before launch.

## UX Example

```text
SIGHT & SOUND 100

Unlock this Canon — €0.99

or

Get all Canons with Reel Canon Pro
```

---

# 13. Canon Discovery

A major long-term product surface is an **Explore Canons** area.

Example:

```text
Explore Canons
```

Cards:

```text
1001 Movies
421 / 1001

IMDb Top 250
93 / 250

Best Picture Winners
34 / 98

Sight & Sound 100
18 / 100

Sci-Fi Essentials
27 / 100
```

A user should feel as though they own a growing **library of film albums**.

Each Canon can have:

- cover
- editorial description
- film count
- progress
- difficulty
- completion badge
- premium status
- source / curator
- release / revision date

---

# 14. Edition Support

Some source lists change over time.

For example, different editions of a book may add and remove films.

The architecture should support multiple editions from the beginning, even if the MVP exposes only one.

This allows future features such as:

```text
Included in editions 2003–2021
```

or:

```text
Added: 2003
Removed: 2026
```

A film should therefore not have a simple field like:

```text
is_1001 = true
```

Instead, list membership belongs in a relation.

---

# 15. Design System

## Visual Direction

Reel Canon should combine:

- editorial typography
- premium spacing
- muted surfaces
- cinematic imagery
- subtle motion
- Material Design interaction quality

## Suggested Palette

Initial direction:

```text
Paper       #F4F1EA
Ink         #171717
Muted       #77736C
Border      #D8D3C9
Accent      #A03027
```

The accent color should be used sparingly.

Film posters should provide most of the visual color.

## Typography

Suggested direction:

### Editorial / Headings

A serif font such as:

- Newsreader
- similar editorial serif

### UI / Metadata

A modern sans serif such as:

- Inter
- similar neutral UI font

## UI Principles

Prefer:

- whitespace
- thin borders
- typography
- subtle transitions
- large imagery
- clean grids

Avoid:

- excessive shadows
- rounded cards everywhere
- bright gradients
- Netflix-style carousels as the main visual language
- generic Material styling
- overly dense dashboards

---

# 16. Material Design Usage

Modern Material Design patterns may be used as a UX foundation.

Useful Material-style primitives:

- dialogs
- bottom sheets
- menus
- selects
- chips
- snackbars
- tooltips
- form controls
- navigation patterns

However, Material Design must be heavily customized.

Reel Canon should not visually feel like a stock Google application.

Tailwind is the primary styling system.

---

# 17. Technical Stack

## Frontend

- React
- Vite
- Single Page Application
- React Router
- Tailwind CSS
- modern Material Design-inspired components
- TanStack Query

## Backend

- Go
- REST API
- JSON

## Database

- PostgreSQL

## Authentication

OAuth / OIDC based authentication for:

- Google
- Apple

The exact authentication provider / implementation remains open.

## State Strategy

Avoid introducing Redux unless a concrete need arises.

Recommended split:

### Server State

Use:

- TanStack Query

Examples:

- films
- filters
- Canon progress
- user progress
- achievements
- streaming availability

### Local UI State

Use:

- React hooks
- Context where appropriate

Examples:

- open dialog
- active sheet
- temporary filter state
- navigation UI

---

# 18. High-Level Architecture

```text
React SPA
│
├── React Router
│
├── Tailwind CSS
│
├── Material-inspired UI components
│
├── TanStack Query
│
└── REST API
       │
       ▼
     Go Backend
       │
       ├── Auth
       ├── Movies
       ├── Canons
       ├── User Progress
       ├── Search / Filters
       ├── Achievements
       ├── Purchases / Entitlements
       └── Streaming
              │
              ▼
          PostgreSQL
```

Possible external systems:

```text
Go Backend
   │
   ├── Movie metadata provider
   ├── Streaming availability provider
   ├── Authentication provider
   └── Payment provider
```

---

# 19. Suggested Routes

Initial frontend route structure:

```text
/
```

Home / dashboard.

```text
/movies
```

Full searchable film library.

```text
/movies/:id
```

Film detail.

```text
/canons
```

Canon discovery.

```text
/canons/:canonId
```

Canon detail / overview.

```text
/canons/:canonId/collection
```

Sticker album / progress.

Optional decade route:

```text
/canons/:canonId/collection/:decade
```

Additional future routes:

```text
/achievements
/profile
/settings
/pro
```

---

# 20. Suggested Backend Modules

Possible Go package organization:

```text
/cmd/api

/internal
    /auth
    /movies
    /canons
    /users
    /progress
    /achievements
    /entitlements
    /streaming
    /search
    /database
```

Prefer a relatively lightweight Go backend.

Avoid introducing a large framework without a clear benefit.

---

# 21. Data Model

The following is a conceptual starting point rather than a final SQL schema.

## movies

```text
id
tmdb_id
imdb_id
title
original_title
release_year
runtime_minutes
overview
poster_url
imdb_rating
created_at
updated_at
```

## directors

```text
id
name
```

## movie_directors

```text
movie_id
director_id
```

## genres

```text
id
name
```

## movie_genres

```text
movie_id
genre_id
```

## countries

```text
id
name
code
```

## movie_countries

```text
movie_id
country_id
```

## canons

```text
id
slug
name
description
cover_url
source
is_free
is_active
created_at
updated_at
```

## canon_editions

```text
id
canon_id
name
year
created_at
```

## canon_movies

```text
canon_id
edition_id
movie_id
position
```

`position` is optional and useful for ranked lists such as IMDb Top 250.

## users

```text
id
created_at
updated_at
```

Authentication provider information may live in a dedicated auth system rather than directly in this table.

## user_movies

```text
user_id
movie_id
watched
watched_at
watchlist
favorite
personal_rating
notes
updated_at
```

## user_canons

```text
user_id
canon_id
unlocked
purchased_at
```

This may eventually be replaced or supplemented by a generic entitlement system.

## achievements

```text
id
slug
name
description
type
is_pro
```

## user_achievements

```text
user_id
achievement_id
unlocked_at
```

## streaming_availability

```text
movie_id
country_code
provider
availability_type
external_url
last_checked_at
```

## subscriptions

Possible later model:

```text
user_id
plan
status
provider
provider_subscription_id
started_at
expires_at
```

## entitlements

A more flexible future model:

```text
id
user_id
type
resource_id
source
created_at
expires_at
```

Examples:

```text
type = canon
resource_id = sight-and-sound-100

type = pro
resource_id = null
```

This allows both:

- one-time Canon purchases
- Pro subscription access

---

# 22. Search & Filtering

Search should ultimately be backed by the Go API.

Example:

```http
GET /api/movies
```

Possible query parameters:

```text
q=
watched=
year_from=
year_to=
decade=
genre=
director=
imdb_min=
canon=
streaming_provider=
sort=
page=
```

Example:

```text
/api/movies?canon=1001-movies&watched=false&decade=1970&imdb_min=7.5
```

Filtering logic should live primarily in the backend so that:

- web
- mobile web
- future native applications

can reuse the same API.

---

# 23. API Direction

Initial API style:

- REST
- JSON
- versioned where useful

Example endpoints:

```text
GET    /api/movies
GET    /api/movies/:id

GET    /api/canons
GET    /api/canons/:id
GET    /api/canons/:id/movies

GET    /api/me
GET    /api/me/progress

PUT    /api/me/movies/:movieId
GET    /api/me/canons/:canonId/progress

GET    /api/me/achievements
GET    /api/movies/:movieId/streaming
```

The exact API contract should be designed before implementation starts.

---

# 24. Repository Structure

Use a monorepo.

Suggested structure:

```text
reel-canon/
│
├── frontend/
│   ├── src/
│   ├── public/
│   ├── package.json
│   └── vite.config.*
│
├── backend/
│   ├── cmd/
│   ├── internal/
│   └── go.mod
│
├── database/
│   ├── migrations/
│   └── seeds/
│
├── scripts/
│
├── docs/
│
├── docker-compose.yml
│
└── README.md
```

Potential future folders:

```text
/contracts
/design
/infrastructure
```

---

# 25. Responsive Strategy

The MVP is **Web First**.

Priority:

1. desktop web
2. mobile web
3. tablet
4. PWA
5. native wrapper / app

The core application should be fully responsive from the start.

## Desktop

Ideal for:

- library browsing
- filters
- album overview
- large poster grids
- detailed statistics

## Mobile

Ideal for:

- progress updates
- film discovery
- quick search
- watchlist
- swipe-based decade navigation
- “what can I stream tonight?”

Mobile should not be a reduced afterthought.

---

# 26. Future App Strategy

Native applications are a later phase.

Possible path:

## Phase 1

Responsive React SPA.

## Phase 2

PWA improvements:

- installability
- caching
- home-screen experience
- possibly offline basics

## Phase 3

Evaluate native packaging or a dedicated native frontend.

The backend API should remain independent of frontend technology.

---

# 27. Product Phases

## Phase 1 — MVP

Focus only on the core product.

Deliver:

- Reel Canon branding
- responsive React SPA
- initial 1001 Movies Canon
- movie data
- film library
- search
- filters
- film detail
- watched / unwatched tracking
- progress
- sticker album
- authentication
- cloud sync

Explicitly avoid overloading the MVP with:

- payments
- subscriptions
- native apps
- social network features
- complex reviews
- streaming licensing
- advanced achievements

## Phase 2 — Collection Platform

Add:

- multiple Canons
- Canon discovery
- edition support
- user Canon library
- individual Canon unlocks
- more statistics

## Phase 3 — Reel Canon Pro

Add:

- streaming availability
- streaming filtering
- advanced achievements
- richer statistics
- completion recommendations
- challenges
- Pro subscription

## Phase 4 — Apps & Personalization

Possible additions:

- PWA
- native apps
- custom Canons
- friends / social elements
- recommendation engine
- richer personal profiles
- notifications
- streaming alerts

---

# 28. Potential Future Features

Ideas worth keeping available but **not** part of the MVP:

- personal movie ratings
- viewing notes
- custom lists / Canons
- public profiles
- friends
- completion leaderboards
- yearly viewing recap
- “film passport”
- recommendations based on missing Canon films
- streaming availability alerts
- “leaving streaming soon”
- notifications when a missing film becomes available
- shareable completion cards
- shareable decade albums
- export / import
- Letterboxd import
- IMDb rating import
- watch-history import
- multiple regional streaming profiles
- family profiles
- themed temporary Canons
- editorial Reel Canon Originals

---

# 29. Important Product Principles

## 1. Completion should feel rewarding

Reel Canon is fundamentally about the satisfaction of completing meaningful collections.

## 2. Progress should be visual

The sticker-album concept is a key differentiator.

## 3. The product should remain calm

Gamification should motivate without turning the product into a noisy game.

## 4. Metadata is secondary to the journey

Reel Canon is not trying to replace IMDb or TMDb.

The value is:

- curation
- progress
- collection
- discovery
- context

## 5. One watched state, many Canons

User film state belongs to the film, not the Canon.

## 6. Build for multiple Canons from day one

Do not hard-code the system around 1001 Movies.

## 7. Streaming is premium utility

Streaming data should enhance the core product rather than define it.

## 8. Web first

The MVP should prioritize a polished responsive web experience over premature native development.

---

# 30. Open Decisions

The following still need to be decided.

## Product

- exact initial 1001 Movies edition
- whether watchlist is included in MVP
- whether personal ratings are MVP or later
- exact free / Pro split
- pricing
- whether individual Canon purchases are permanent
- which additional Canons launch first
- handling of copyrighted list / book branding in product copy

## Design

- final typography
- final palette
- album layout
- degree of blur for unseen films
- poster treatment
- exact Material component library, if any
- motion design

## Technical

- authentication provider / implementation
- database access layer in Go
- Go router
- migration tooling
- hosting
- CDN
- image hosting / caching
- source of movie metadata
- source of IMDb ratings
- streaming data provider
- payment provider
- entitlement implementation

## Legal / Data

- rights around list names and branding
- movie poster licensing / API terms
- metadata provider terms
- commercial streaming-data licensing
- privacy policy
- account deletion
- GDPR compliance
- payment / subscription rules

---

# 31. Immediate Next Steps

Recommended order:

1. finalize MVP scope
2. choose the initial 1001 Movies edition
3. design the canonical data model
4. define the first REST API contract
5. source and normalize the initial film dataset
6. create the design tokens
7. design desktop and mobile wireframes
8. prototype the sticker album
9. scaffold the monorepo
10. build the first vertical slice

## Recommended First Vertical Slice

A useful first end-to-end feature:

```text
Open Reel Canon
→ browse 1001 Movies
→ open a film
→ mark it as watched
→ return to collection
→ poster changes from monochrome to color
→ progress count increases
```

This touches:

- React SPA
- React Router
- Tailwind
- API
- Go backend
- PostgreSQL
- user state
- collection progress
- the core visual identity

and proves the central product idea early.

---

# 32. Working Product Summary

**Reel Canon** is a premium film-completion platform built around curated cinema Canons.

The free product begins with the 1001 Movies collection.

Users progressively “fill” an elegant film album by watching movies. Unseen films remain muted and monochrome; watched films become full-color collectibles.

Over time, users can expand their library with additional Canons, achievements, advanced statistics, streaming availability, and premium discovery tools.

The long-term vision is not simply:

> “Which films have I watched?”

but:

> **Which parts of cinema have I explored — and what should I discover next?**
