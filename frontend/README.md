# Spill frontend

The frontend is an independent React, Vite, and TypeScript application. It only calls the existing Spill API; it does not add or change backend routes.

## Run locally

```powershell
cd frontend
npm install
npm run dev
```

By default, the browser calls `/api/v1` on the Vite origin. Vite proxies that prefix to `http://localhost:8080`, strips `/api/v1` for the backend (which also mounts the routes at root), and rewrites the refresh cookie path to `/api/v1/auth`. This keeps API fetches same-origin during local development. Set `VITE_API_PROXY_TARGET` to use another backend address.

For deployments, set `VITE_API_BASE_URL` to the API base URL. A cross-origin deployment needs credentialed CORS for the exact frontend origin and the required request headers/methods. The backend sets the refresh cookie path to `/auth`; a proxy serving it under `/api/v1` must rewrite the cookie path accordingly. A genuinely cross-site setup also needs a cookie policy that permits that deployment; the current cookie is `SameSite=Strict`.

## API behavior and known boundaries

- Access tokens are held in memory. Protected calls send them as Bearer tokens. Refresh uses a shared in-flight promise, rotates once, and retries an unauthorized API request at most once.
- Home displays the followed-user feed and the current user’s posts as separate independently paginated streams. Their cursors are not combined, and the UI does not claim a globally consistent ordering.
- Share creates a share and optional quote through `POST /posts/{id}/share`. The current API has no share-list, share-history, shared-feed, or undo-share endpoint; the frontend does not fabricate those features.
- Follow reads do not expose whether the current user already follows a profile. The profile view starts in an unknown state and only reflects the state returned by a follow/unfollow action.
- Image upload follows the API flow: create asset, PUT the file to the returned presigned URL, then complete the asset. The uploader sends the file MIME type as `Content-Type` and does not send API cookies or Bearer credentials to object storage. The signing implementation does not pass a content-type constraint when generating the URL. Successful browser uploads still depend on the live storage CORS policy and signed URL behavior; validate against the configured storage before treating deployment uploads as verified.
- The API contract is inconsistent about comment length. The frontend does not impose a character limit; it reports the API’s validation error.

## Commands

```powershell
npm run lint
npm run build
```
