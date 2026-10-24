# One-time setup: give Identity its GitHub app (owner, ~2 minutes)

Identity's **Connect GitHub** step uses GitHub's official sign-in: the person sees a short code, approves on github.com, and Identity gets permission to **read** the Thought Processor repository. For that, GitHub needs to know which app is asking. You register that app once; everyone using this copy of Identity then just clicks **Connect GitHub**.

The **Client ID** you get is public (it is not a password). There is **no client secret** in this flow — do not create or paste one.

## Steps

1. On github.com, open **Settings → Developer settings → GitHub Apps → New GitHub App**.
2. Fill in:
   - **GitHub App name:** `Identity setup (Arameair)` (any unused name)
   - **Homepage URL:** `https://github.com/Arameair/identity`
   - **Callback URL:** leave empty
   - **Enable Device Flow:** ✅ **check this** (required)
   - **Webhook → Active:** ☐ uncheck
3. **Permissions → Repository permissions:**
   - **Contents:** *Read-only*
   - **Metadata:** *Read-only* (GitHub sets this automatically)
   - Leave everything else at *No access*.
4. **Where can this GitHub App be installed?** → *Only on this account*. Click **Create GitHub App**.
5. On the app's page, copy the **Client ID** (starts with `Iv`).
6. Click **Install App** → install on **Arameair** → *Only select repositories* → choose **Thought-Processor** → Install.
7. Put the Client ID next to the Identity program in a file called `github-app.json`:

   ```json
   { "client_id": "Iv23liXXXXXXXXXXXXXX" }
   ```

   (or start Identity with the environment variable `IDENTITY_GITHUB_CLIENT_ID`). Restart Identity.

## What this permits

- Identity can read **who** signed in and **read** the Thought-Processor repository's information, only for repositories where the app is installed **and** the signed-in person already has access.
- It cannot change code, create repositories, or see other repositories.
- The sign-in is kept in Identity's memory only; closing Identity forgets it.

## If something goes wrong

| Identity says | Fix |
| --- | --- |
| *hasn't been linked to a GitHub app yet* | `github-app.json` missing or empty |
| *GitHub sign-in isn't switched on for this app yet* | Step 2: tick **Enable Device Flow** |
| *GitHub doesn't recognise this app's Client ID* | Re-copy the Client ID (step 5) |
| *the account … can't see Arameair/Thought-Processor* | Install the app on that repository (step 6), and make sure that person is a collaborator |

**Note:** Project Controller downloads the code with **this computer's own git sign-in**, separately from Identity. If Controller says it can't download, sign this computer's git in to GitHub (for example with GitHub Desktop or `gh auth login`).
