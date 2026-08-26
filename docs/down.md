# down

Stops the environment and removes its containers.

```bash
hm down        # containers go, data stays
hm down -v     # volumes go too, and the database with them
```

## The one letter that matters

`hm down` removes containers. Everything in the volumes — the database, the search index, the
session store — is still there, and `hm start` brings the environment back with its data.

`hm down -v` **deletes the volumes**. That is the database gone, with no way back. It is one letter
of difference, and an environment on this machine was lost exactly that way while this version was
being written.

So `-v` now asks, names the volumes it is about to delete, and offers the cheap way out:

```console
$ hm down -v

This deletes the volumes of 'my-project', and the database with them:

  my-project_dbdata
  my-project_searchdata
  my-project_redisdata

What should happen?
  1) Save a database snapshot, then destroy
  2) Destroy without saving
  3) Cancel
```

Saving first is the default because it is the answer nobody regrets. It uses
[`hm db snapshot`](db.md), so the copy lives outside the project and survives what you are about
to do.

Copies are not taken automatically: a project that is destroyed on purpose several times a day
would end up with a pile of them that nobody looks at.

## Without a terminal

In a script, in CI, or with `--yes`, nothing is asked and `-v` destroys as it always did. Somebody
who automates `hm down -v` wrote it on purpose.

## `docker-stop-all`

[`hm docker-stop-all`](docker-stop-all.md) stops **every container on the machine**, including
other people's projects and things unrelated to Dockergento. It now says how many it will stop and
how many are not yours, and asks. Nothing is destroyed — what is being protected is somebody else's
work in progress.
