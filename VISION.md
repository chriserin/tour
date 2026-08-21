Reviewing code can be challenging, especially a complicated change set across a
disparate set of files.

An app gives you a tour of the code changes for a particular diff or feature.
This works by defining slides that contain text and a file:line location used by
neovim's remote capabilities to direct the editor to the correct location for
the correct slide. These slides should be plain text and should be definable
by human or AI. They should take advantage of the linear nature of the slide
show to order the slides deterministically so that a narrative can be
constructed around the feature or change set.
