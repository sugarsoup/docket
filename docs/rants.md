scaling workers is stupid, just use rpc services for things that have to be distributed
long polling is problematic in a number of environments and infra setups
most steps dont need persistence
versioning becomes a real nightmare
the UI doesnt actually work at scale 
the cli doesnt work at scale, can't make huge numbers of changes
multiple services is too many
each service is too expensive
too many metrics
too expensive
too many parameters to tune
dont identify things with strings, identify them with protos
temporal pays every other kind of cost to prevent having to define a data schema in a database

aspirational: prevent data races
https://gaultier.github.io/blog/a_million_ways_to_data_race_in_go.html

this kaggle is very similar to what i want for the further example
https://www.kaggle.com/code/muhammedaliyilmazz/imdb-movie-data-analysis-and-ml-modeling

2 yo list of bioinformatics software written in golang
 https://github.com/dissipative/awesome-bio-go

 