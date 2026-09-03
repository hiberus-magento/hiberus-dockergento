# Design

## The selector belongs to the terminal, not to the engine

The engine says "I need one of these answers"; who asks and how is not its business. So it is a
function the command line hands in, beside the one that asks for a value and the one that draws
the spinner — which is what lets the same use case answer an HTTP request later without a terminal
anywhere in it.

That also settles where the parts live: moving through a list and drawing it are two functions of
an index and some strings, and they are tested without a terminal at all. What cannot be tested
that way — raw mode, the escape sequences, the redraw — is the thin part around them.

## Escape does nothing

Deliberately, and it is the same reason the shell implementation gives: a caller reads the answer
and acts on it, so a cancel that returned an empty one would have it carry on with nothing chosen.
For `hm down -v` that is the wrong branch of a destructive question. Ctrl-C still does what Ctrl-C
does everywhere.

## Asking only when there is something to lose

The question is asked when volumes are being deleted and there are volumes to delete. A project
whose volumes were already removed is not asked anything, and neither is a run that is not
interactive: the flag was explicit, and a command that hung waiting for an answer nobody can give
would be worse than the deletion.
