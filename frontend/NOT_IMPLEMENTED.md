# Not implemented

Frontend gaps to work through next:

1. Follows page
   - List followers
   - List following
   - Follow an account
   - Unfollow an account
   - Search or resolve remote accounts

2. Inbox page
   - Show incoming follows, mentions, updates, deletes, accepts, and rejects
   - Filter by activity type
   - Show raw details behind an explicit inspect action

3. Outbox page
   - Show sent activities
   - Show recipients or audience
   - Link sent posts to delivery state

4. Delivery page
   - Show delivery queue
   - Show attempts, next retry, delivered time, and last error
   - Retry or cancel failed delivery when backend supports it

5. Posts page
   - Refresh timeline after posting without manual reload
   - Add visibility selection if backend supports it
   - Add delete post if backend supports it
   - Add better empty/error states after real data testing

6. Compatibility page
   - Removed until there is a concrete user-facing purpose for it

7. Navigation and shell
   - Improve mobile navigation after testing on device widths
   - Add active account switcher only if multiple accounts become supported

8. Auth/session
   - Decide whether refresh tokens are supported
   - Handle expired tokens explicitly
   - Add revocation/sign-out endpoint if backend supports it
