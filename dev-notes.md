# Next steps

I've extended the store, the point of the store now is that it will store everything for a given height.
It stores the raw message so users of the lib have no need to provide their own storage for that. And it also performs equivocation detection.
Since the add method returns errors it still needs to be used outside of the algorithm.

Next steps would be to make a testing framework to remove boilerplate for the more integration type tests and to add a bunch more tests, the store should also be tested.
