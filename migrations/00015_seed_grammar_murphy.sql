-- +goose Up
-- Grammar questions organised by Raymond Murphy "Essential/English Grammar in Use" units.
-- Category names follow Murphy's section titles so they can be used for adaptive selection.
-- Coverage: ~100 questions per level for A1, A2, B1; ~30 each for B2, C1, C2.

INSERT INTO grammar_questions (level, category, prompt, options, answer) VALUES

-- ============================================================
-- A1 — Elementary Grammar in Use
-- ============================================================

-- am / is / are
('A1', 'am/is/are', '___ your brother a student?', '["Is","Are","Am","Be"]', 'Is'),
('A1', 'am/is/are', 'I ___ not feeling well today.', '["are","is","am","be"]', 'am'),
('A1', 'am/is/are', 'Where ___ your parents from?', '["is","am","are","be"]', 'are'),
('A1', 'am/is/are', 'She ___ a doctor.', '["am","are","is","be"]', 'is'),
('A1', 'am/is/are', '___ they ready?', '["Is","Am","Are","Be"]', 'Are'),
('A1', 'am/is/are', 'What ___ your name?', '["are","am","is","be"]', 'is'),
('A1', 'am/is/are', 'We ___ very happy about the news.', '["is","am","are","be"]', 'are'),
('A1', 'am/is/are', 'The café ___ closed on Sundays.', '["are","be","am","is"]', 'is'),
('A1', 'am/is/are', 'I ___ twenty years old.', '["are","is","be","am"]', 'am'),
('A1', 'am/is/are', '___ this the right train?', '["Are","Am","Is","Be"]', 'Is'),

-- Present Simple
('A1', 'Present Simple', 'She ___ to work by bus every day.', '["go","is going","went","goes"]', 'goes'),
('A1', 'Present Simple', 'He ___ not like spicy food.', '["don''t","isn''t","aren''t","doesn''t"]', 'doesn''t'),
('A1', 'Present Simple', 'What time ___ the bank open?', '["do","is","did","does"]', 'does'),
('A1', 'Present Simple', 'They ___ in London.', '["lives","is living","lived","live"]', 'live'),
('A1', 'Present Simple', 'It rarely ___ in this city.', '["rain","is raining","rained","rains"]', 'rains'),
('A1', 'Present Simple', '___ she work on Saturdays?', '["Do","Is","Did","Does"]', 'Does'),
('A1', 'Present Simple', 'We ___ English at school.', '["studies","are studying","studied","study"]', 'study'),
('A1', 'Present Simple', 'He always ___ his teeth before bed.', '["brush","is brushing","brushed","brushes"]', 'brushes'),
('A1', 'Present Simple', 'The sun ___ in the west.', '["set","is setting","have set","sets"]', 'sets'),
('A1', 'Present Simple', '___ you enjoy cooking?', '["Does","Are","Is","Do"]', 'Do'),

-- Present Continuous
('A1', 'Present Continuous', 'Look! She ___ her homework.', '["does","do","is doing","was doing"]', 'is doing'),
('A1', 'Present Continuous', 'We ___ to the cinema tonight.', '["go","went","are going","goes"]', 'are going'),
('A1', 'Present Continuous', 'Please be quiet! The baby ___.', '["sleep","sleeps","is sleeping","sleeping"]', 'is sleeping'),
('A1', 'Present Continuous', 'He ___ the newspaper at the moment.', '["reads","read","is reading","has read"]', 'is reading'),

-- Present Simple vs Present Continuous
('A1', 'Present Simple vs Present Continuous', 'Look! It ___ outside.', '["rains","rained","raining","is raining"]', 'is raining'),
('A1', 'Present Simple vs Present Continuous', 'I ___ coffee every morning.', '["am drinking","drank","have drunk","drink"]', 'drink'),
('A1', 'Present Simple vs Present Continuous', 'He ___ on the phone right now.', '["talks","talked","talk","is talking"]', 'is talking'),
('A1', 'Present Simple vs Present Continuous', 'She usually ___ to bed at 10 pm.', '["is going","went","go","goes"]', 'goes'),
('A1', 'Present Simple vs Present Continuous', 'Why ___ you wearing a coat? It is warm!', '["do","did","have","are"]', 'are'),
('A1', 'Present Simple vs Present Continuous', 'I ___ this is not a good idea. (opinion)', '["am thinking","thought","have thought","think"]', 'think'),
('A1', 'Present Simple vs Present Continuous', 'Water ___ at 100 degrees Celsius.', '["is boiling","boiled","boil","boils"]', 'boils'),
('A1', 'Present Simple vs Present Continuous', 'The kids ___ in the garden at the moment.', '["play","played","plays","are playing"]', 'are playing'),

-- there is / there are
('A1', 'there is / there are', '___ a problem with the printer.', '["There are","It is","They are","There is"]', 'There is'),
('A1', 'there is / there are', '___ any milk in the fridge?', '["Does there","There is","There are","Is there"]', 'Is there'),
('A1', 'there is / there are', '___ some chairs in the corner.', '["It is","There is","These are","There are"]', 'There are'),
('A1', 'there is / there are', 'How many rooms ___ in your flat?', '["there are","is there","there is","are there"]', 'are there'),
('A1', 'there is / there are', '___ a lot of traffic today.', '["There are","It is","They are","There is"]', 'There is'),
('A1', 'there is / there are', '___ three windows in the room.', '["There is","It has","These are","There are"]', 'There are'),
('A1', 'there is / there are', '___ a good film on TV tonight?', '["Are there","Has there","There is","Is there"]', 'Is there'),
('A1', 'there is / there are', '___ no sugar left in the bowl.', '["It is","There are","There was","There is"]', 'There is'),

-- Have / Have got
('A1', 'Have/Have got', 'She ___ a beautiful garden.', '["have","has got","have got","haves"]', 'has got'),
('A1', 'Have/Have got', '___ you got a pen I could borrow?', '["Have","Has","Do","Does"]', 'Have'),
('A1', 'Have/Have got', 'He ___ any brothers or sisters.', '["doesn''t have","hasn''t got","don''t has","hasn''t have"]', 'hasn''t got'),

-- Some / Any
('A1', 'Some/Any', 'I''d like ___ milk in my coffee, please.', '["any","some","no","a"]', 'some'),
('A1', 'Some/Any', 'Are there ___ eggs in the fridge?', '["some","no","any","a"]', 'any'),
('A1', 'Some/Any', 'There isn''t ___ sugar left.', '["some","no","any","a"]', 'any'),
('A1', 'Some/Any', 'Can I have ___ water, please?', '["any","a","some","no"]', 'some'),

-- Articles: a / an
('A1', 'Articles: a/an', 'She is ___ engineer.', '["a","the","-","an"]', 'an'),
('A1', 'Articles: a/an', 'He eats ___ apple every day.', '["a","the","some","an"]', 'an'),
('A1', 'Articles: a/an', 'I saw ___ interesting film yesterday.', '["a","the","-","an"]', 'an'),
('A1', 'Articles: a/an', 'Is there ___ bank near here?', '["the","an","-","a"]', 'a'),
('A1', 'Articles: a/an', 'He has ___ old car.', '["a","the","-","an"]', 'an'),
('A1', 'Articles: a/an', 'She wants to be ___ doctor.', '["an","the","-","a"]', 'a'),
('A1', 'Articles: a/an', '___ umbrella is a useful thing to have.', '["A","The","-","An"]', 'An'),
('A1', 'Articles: a/an', 'He works in ___ office downtown.', '["a","the","-","an"]', 'an'),
('A1', 'Articles: a/an', 'Can I have ___ cup of tea?', '["an","the","-","a"]', 'a'),
('A1', 'Articles: a/an', 'She married ___ architect last year.', '["a","the","-","an"]', 'an'),

-- Prepositions of time
('A1', 'Prepositions of time', 'The meeting is ___ Monday.', '["in","at","on","to"]', 'on'),
('A1', 'Prepositions of time', 'She was born ___ 1995.', '["on","in","at","to"]', 'in'),
('A1', 'Prepositions of time', 'The train arrives ___ 6 o''clock.', '["in","on","at","by"]', 'at'),
('A1', 'Prepositions of time', 'I always have a sandwich ___ lunchtime.', '["in","at","on","during"]', 'at'),

-- Prepositions of place
('A1', 'Prepositions of place', 'Put the cups ___ the shelf.', '["in","at","under","on"]', 'on'),
('A1', 'Prepositions of place', 'She lives ___ Italy.', '["on","at","by","in"]', 'in'),
('A1', 'Prepositions of place', 'I was born ___ a small village.', '["on","at","by","in"]', 'in'),
('A1', 'Prepositions of place', 'He is waiting ___ the cinema.', '["in","on","by","at"]', 'at'),
('A1', 'Prepositions of place', 'The dog is sleeping ___ the table.', '["on","in","at","under"]', 'under'),
('A1', 'Prepositions of place', 'Is there a shop ___ this street?', '["in","at","by","on"]', 'on'),
('A1', 'Prepositions of place', 'The pictures are ___ the wall.', '["in","at","under","on"]', 'on'),
('A1', 'Prepositions of place', 'She sat ___ the window and watched the rain.', '["on","in","at","by"]', 'by'),
('A1', 'Prepositions of place', 'We met ___ the park.', '["on","at","by","in"]', 'in'),

-- Question words
('A1', 'Question words', '___ is your birthday?', '["Who","Where","What","When"]', 'When'),
('A1', 'Question words', '___ does she live?', '["What","When","Who","Where"]', 'Where'),
('A1', 'Question words', '___ is your teacher?', '["What","When","Where","Who"]', 'Who'),
('A1', 'Question words', '___ colour is your car?', '["When","Where","Who","What"]', 'What'),
('A1', 'Question words', '___ much does this cost?', '["What","When","Where","How"]', 'How'),
('A1', 'Question words', '___ are you late?', '["Who","Where","When","Why"]', 'Why'),
('A1', 'Question words', '___ did you do at the weekend?', '["Where","When","Who","What"]', 'What'),
('A1', 'Question words', '___ long is the film?', '["When","What","Which","How"]', 'How'),

-- Adjectives and adverbs
('A1', 'Adjectives and adverbs', 'She speaks English very ___.', '["good","well","goodly","nice"]', 'well'),
('A1', 'Adjectives and adverbs', 'He drives ___.', '["careful","carefully","careless","caringly"]', 'carefully'),
('A1', 'Adjectives and adverbs', 'Run ___! We''re late.', '["quick","quicker","quickly","more quick"]', 'quickly'),

-- ============================================================
-- A2 — Elementary to Pre-Intermediate
-- ============================================================

-- Past Simple — irregular verbs
('A2', 'Past Simple (irregular)', 'She ___ a letter to her friend last week.', '["writed","write","written","wrote"]', 'wrote'),
('A2', 'Past Simple (irregular)', 'We ___ to Spain for our holiday.', '["drived","drive","drove","driven"]', 'drove'),
('A2', 'Past Simple (irregular)', 'He ___ the news and felt very sad.', '["heared","hear","heard","has heard"]', 'heard'),
('A2', 'Past Simple (irregular)', 'I ___ the answer but I forgot it.', '["knowed","knew","know","known"]', 'knew'),

-- Past Continuous
('A2', 'Past Continuous', 'She ___ a shower when the phone rang.', '["had","has had","is having","was having"]', 'was having'),
('A2', 'Past Continuous', 'They ___ dinner at 7 pm last night.', '["had","have","are having","were having"]', 'were having'),
('A2', 'Past Continuous', 'I ___ across the road when I saw him.', '["walk","have walked","walked","was walking"]', 'was walking'),
('A2', 'Past Continuous', 'He fell asleep while he ___ the report.', '["reads","had read","read","was reading"]', 'was reading'),
('A2', 'Past Continuous', 'What ___ you doing at midnight?', '["did","had","have","were"]', 'were'),
('A2', 'Past Continuous', 'The sun ___ brightly when we arrived.', '["shone","shines","had shone","was shining"]', 'was shining'),
('A2', 'Past Continuous', 'We ___ about you when you arrived.', '["talked","talk","are talking","were talking"]', 'were talking'),
('A2', 'Past Continuous', 'She ___ while he was speaking.', '["laughed","laughs","has laughed","was laughing"]', 'was laughing'),
('A2', 'Past Continuous', 'He hurt his back while he ___ the furniture.', '["moves","moved","had moved","was moving"]', 'was moving'),
('A2', 'Past Continuous', '___ it raining when you got up?', '["Were","Did","Is","Was"]', 'Was'),

-- Present Perfect
('A2', 'Present Perfect', '___ you ever been to Japan?', '["Did","Have","Do","Were"]', 'Have'),
('A2', 'Present Perfect', 'I have never ___ sushi before.', '["eat","ate","eating","eaten"]', 'eaten'),
('A2', 'Present Perfect', 'She has ___ many countries over the years.', '["visited","visit","visits","visiting"]', 'visited'),
('A2', 'Present Perfect', 'We have just ___ the good news.', '["hear","heard","hearing","hears"]', 'heard'),

-- Going to
('A2', 'Going to', 'She ___ study medicine. (She has already decided)', '["goes to","would","will","is going to"]', 'is going to'),
('A2', 'Going to', 'Look at those clouds! It ___ snow.', '["goes to","snows","will","is going to"]', 'is going to'),
('A2', 'Going to', 'They ___ open a new restaurant next year.', '["going","will","go to","are going to"]', 'are going to'),
('A2', 'Going to', 'Be careful! You ___ fall!', '["were going","go to","will","are going to"]', 'are going to'),
('A2', 'Going to', 'Are you ___ see the doctor tomorrow?', '["going","will","go","going to"]', 'going to'),
('A2', 'Going to', 'I ___ not going to argue about this.', '["will","is","are","am"]', 'am'),
('A2', 'Going to', 'They ___ going to build a new school here.', '["is","will","shall","are"]', 'are'),
('A2', 'Going to', 'We ___ going to be late if we don''t hurry.', '["is","will","shall","are"]', 'are'),
('A2', 'Going to', 'She ___ going to arrive at noon.', '["are","am","will","is"]', 'is'),
('A2', 'Going to', 'It ___ going to be a beautiful day.', '["are","am","will","is"]', 'is'),

-- will — future
('A2', 'will (future)', 'I''m cold. I ___ close the window.', '["am going to","will","going to","shall to"]', 'will'),
('A2', 'will (future)', 'Don''t worry, everything ___ be fine.', '["is going to","will","shall","would"]', 'will'),
('A2', 'will (future)', 'A: I need some help. B: I ___ give you a hand.', '["am going to","going to","will","would"]', 'will'),

-- Comparatives
('A2', 'Comparatives', 'She is ___ her sister. (tall)', '["tall than","the tallest","more tall than","taller than"]', 'taller than'),
('A2', 'Comparatives', 'This book is ___ that one. (interesting)', '["interestinger than","interesting than","most interesting","more interesting than"]', 'more interesting than'),
('A2', 'Comparatives', 'He runs ___ me. (fast)', '["fastest","fast than","more fast than","faster than"]', 'faster than'),
('A2', 'Comparatives', 'Today is ___ yesterday. (cold)', '["coldest","cold than","more cold than","colder than"]', 'colder than'),
('A2', 'Comparatives', 'Which is ___ — the red car or the blue one? (expensive)', '["expensiver","the most expensive","most expensive","more expensive"]', 'more expensive'),
('A2', 'Comparatives', 'He is ___ his brother by two years. (old)', '["more old than","old than","eldest","older than"]', 'older than'),
('A2', 'Comparatives', 'This exercise is ___ the last one. (difficult)', '["difficulter than","difficult than","most difficult","more difficult than"]', 'more difficult than'),
('A2', 'Comparatives', 'She speaks English ___ I do. (good)', '["more good than","gooder than","best","better than"]', 'better than'),
('A2', 'Comparatives', 'This road is ___ that one. (wide)', '["widest","wide than","more wide than","wider than"]', 'wider than'),
('A2', 'Comparatives', 'The weather is ___ today than yesterday. (bad)', '["most bad","badder","worst","worse"]', 'worse'),

-- Superlatives
('A2', 'Superlatives', 'This is ___ restaurant in the city.', '["the best","better","the most good","best"]', 'the best'),
('A2', 'Superlatives', 'It was ___ day of my life.', '["the happiest","the most happy","more happy","happiest"]', 'the happiest'),
('A2', 'Superlatives', 'January is ___ month of the year.', '["the coldest","colder","the more cold","coldest"]', 'the coldest'),
('A2', 'Superlatives', 'She is ___ person I know.', '["more generous","the most generous","generousest","the generousest"]', 'the most generous'),

-- First conditional
('A2', 'First conditional', 'If it rains, we ___ inside.', '["stayed","would stay","stay","will stay"]', 'will stay'),
('A2', 'First conditional', 'She will be upset if you ___ her the truth.', '["didn''t tell","wouldn''t tell","won''t tell","don''t tell"]', 'don''t tell'),
('A2', 'First conditional', 'If he ___ hard, he''ll pass the exam.', '["study","will study","studied","studies"]', 'studies'),
('A2', 'First conditional', 'I ___ you if I need help.', '["call","called","would call","will call"]', 'will call'),
('A2', 'First conditional', 'If they ___ early, we can have breakfast together.', '["arrived","will arrive","would arrive","arrive"]', 'arrive'),
('A2', 'First conditional', 'We ___ if you don''t hurry.', '["are late","be late","would be late","will be late"]', 'will be late'),
('A2', 'First conditional', 'If the weather is nice, she ___ for a walk.', '["goes","would go","went","will go"]', 'will go'),
('A2', 'First conditional', 'He ___ his job unless he works harder.', '["lose","lost","would lose","will lose"]', 'will lose'),
('A2', 'First conditional', 'If you ___ the bus, you''ll miss the meeting.', '["missed","will miss","would miss","miss"]', 'miss'),
('A2', 'First conditional', 'They ___ us if they need anything.', '["told","would tell","tell","will tell"]', 'will tell'),

-- should / ought to
('A2', 'should / ought to', 'You look tired. You ___ go to bed earlier.', '["would","must","might","should"]', 'should'),
('A2', 'should / ought to', 'She ___ see a doctor about that cough.', '["will","would","may","should"]', 'should'),
('A2', 'should / ought to', 'He ___ drive so fast — it''s dangerous.', '["wouldn''t","couldn''t","mustn''t","shouldn''t"]', 'shouldn''t'),
('A2', 'should / ought to', 'You ___ to tell her the truth.', '["should","must","have","ought"]', 'ought'),
('A2', 'should / ought to', 'They ___ be more careful with their money.', '["would","could","might","should"]', 'should'),
('A2', 'should / ought to', 'I think you ___ apologise to him.', '["might","would","could","should"]', 'should'),
('A2', 'should / ought to', '___ I bring anything to the party?', '["Would","Must","Might","Should"]', 'Should'),
('A2', 'should / ought to', 'We ___ leave early to avoid the traffic.', '["would","could","might","should"]', 'should'),

-- must / have to / don't have to
('A2', 'must/have to/don''t have to', 'The sign says No Parking. You ___ park here.', '["mustn''t","don''t have to","needn''t","may"]', 'mustn''t'),
('A2', 'must/have to/don''t have to', 'Sunday is a holiday. We ___ go to work.', '["must","have to","don''t have to","mustn''t"]', 'don''t have to'),
('A2', 'must/have to/don''t have to', 'I''m very tired. I ___ go to bed early tonight.', '["must","mustn''t","don''t have to","should to"]', 'must'),
('A2', 'must/have to/don''t have to', 'The restaurant is always empty — you ___ make a reservation.', '["must","mustn''t","don''t have to","have to"]', 'don''t have to'),

-- would like
('A2', 'would like', '___ you like a cup of tea?', '["Do","Would","Will","Are"]', 'Would'),
('A2', 'would like', 'She ___ to visit Rome one day.', '["would like","likes","would likes","would liked"]', 'would like'),
('A2', 'would like', 'I ___ a table for two, please.', '["want","would like","like","liked"]', 'would like'),

-- Articles: the
('A2', 'Articles: the', '___ Pacific is the largest ocean.', '["A","An","-","The"]', 'The'),
('A2', 'Articles: the', 'She played ___ piano at the concert.', '["a","an","-","the"]', 'the'),
('A2', 'Articles: the', 'He is ___ tallest in the class.', '["a","an","-","the"]', 'the'),
('A2', 'Articles: the', 'Can you pass ___ salt, please?', '["a","an","-","the"]', 'the'),
('A2', 'Articles: the', 'We went to ___ cinema last night.', '["a","an","-","the"]', 'the'),
('A2', 'Articles: the', '___ Earth goes around the Sun.', '["A","An","-","The"]', 'The'),
('A2', 'Articles: the', 'She is learning to play ___ guitar.', '["a","an","-","the"]', 'the'),
('A2', 'Articles: the', 'Is this ___ book you were looking for?', '["a","an","-","the"]', 'the'),

-- too / enough
('A2', 'too / enough', 'It is ___ cold to go swimming.', '["enough","very","so","too"]', 'too'),
('A2', 'too / enough', 'She isn''t old ___ to vote.', '["too","very","so","enough"]', 'enough'),
('A2', 'too / enough', 'The coffee is ___ hot to drink.', '["enough","very","so","too"]', 'too'),
('A2', 'too / enough', 'He doesn''t study ___ to pass.', '["too","very","so","enough"]', 'enough'),
('A2', 'too / enough', 'This bag is ___ heavy for me to carry.', '["enough","very","so","too"]', 'too'),
('A2', 'too / enough', 'Are you experienced ___ for the job?', '["too","very","so","enough"]', 'enough'),
('A2', 'too / enough', 'The box is ___ big to fit in the car.', '["enough","very","so","too"]', 'too'),

-- ============================================================
-- B1 — Pre-Intermediate to Intermediate
-- ============================================================

-- Past Perfect
('B1', 'Past Perfect', 'When I arrived, everyone ___ already left.', '["has","have","had","was"]', 'had'),
('B1', 'Past Perfect', 'She didn''t want to eat because she ___ just had lunch.', '["has","have","had","was"]', 'had'),
('B1', 'Past Perfect', 'I ___ never visited Paris before that trip.', '["have","has","had","was"]', 'had'),
('B1', 'Past Perfect', 'By the time we got to the cinema, the film ___.', '["has started","started","had started","starts"]', 'had started'),

-- Present Perfect Continuous
('B1', 'Present Perfect Continuous', 'She looks tired. She ___ all day.', '["has been working","has worked","had been working","is working"]', 'has been working'),
('B1', 'Present Perfect Continuous', 'How long ___ you been learning English?', '["have","has","had","are"]', 'have'),
('B1', 'Present Perfect Continuous', 'I ___ here since 2010.', '["live","am living","have been living","lived"]', 'have been living'),
('B1', 'Present Perfect Continuous', 'It ___ raining for three hours now.', '["is","was","has been","had been"]', 'has been'),

-- Passive Voice
('B1', 'Passive Voice', 'The Eiffel Tower ___ in 1889.', '["built","is built","was built","has built"]', 'was built'),
('B1', 'Passive Voice', 'English ___ all over the world.', '["speaks","is spoken","spoken","is speaking"]', 'is spoken'),
('B1', 'Passive Voice', 'The documents ___ still being processed.', '["is","was","are","were"]', 'are'),
('B1', 'Passive Voice', 'The new bridge ___ by the end of the year.', '["will finish","will be finished","finishes","is finishing"]', 'will be finished'),

-- Reported Speech
('B1', 'Reported Speech', 'He said "I will call you tomorrow." → He said that he ___ call me the next day.', '["will","would","should","could"]', 'would'),
('B1', 'Reported Speech', '"We are leaving now." → They said they ___ leaving.', '["are","were","would be","will be"]', 'were'),
('B1', 'Reported Speech', '"Do you like pizza?" → She asked me whether I ___ pizza.', '["like","liked","likes","would like"]', 'liked'),
('B1', 'Reported Speech', '"Come early." → He told me ___.', '["come early","to come early","that come early","coming early"]', 'to come early'),

-- Relative Clauses (defining)
('B1', 'Relative Clauses', 'The man ___ lives next door is a doctor.', '["which","who","whose","whom"]', 'who'),
('B1', 'Relative Clauses', 'The car ___ I bought last year has broken down.', '["who","whose","which","whom"]', 'which'),
('B1', 'Relative Clauses', 'She''s the woman ___ husband works at the bank.', '["which","who","whose","whom"]', 'whose'),
('B1', 'Relative Clauses', 'Is there anything ___ I can do to help?', '["which","who","that","whose"]', 'that'),

-- Non-defining relative clauses
('B1', 'Non-defining relative clauses', 'My sister, ___ lives in Rome, is a doctor.', '["which","that","whose","who"]', 'who'),
('B1', 'Non-defining relative clauses', 'The Eiffel Tower, ___ is in Paris, attracts millions of tourists.', '["who","that","whose","which"]', 'which'),
('B1', 'Non-defining relative clauses', 'London, ___ I visited last year, is a wonderful city.', '["where","who","that","which"]', 'which'),
('B1', 'Non-defining relative clauses', 'My neighbour, ___ dog barks all night, never apologises.', '["who","which","that","whose"]', 'whose'),
('B1', 'Non-defining relative clauses', 'The concert, ___ lasted three hours, was amazing.', '["who","that","where","which"]', 'which'),
('B1', 'Non-defining relative clauses', 'Her grandmother, ___ is 90 years old, still works in the garden.', '["which","that","whose","who"]', 'who'),
('B1', 'Non-defining relative clauses', 'The book, ___ I bought last week, is absolutely brilliant.', '["who","whose","that","which"]', 'which'),
('B1', 'Non-defining relative clauses', 'Paris, ___ the fashion industry began, is still a style capital.', '["who","which","that","where"]', 'where'),
('B1', 'Non-defining relative clauses', 'My friend Anna, ___ parents are both musicians, plays the violin.', '["who","which","that","whose"]', 'whose'),
('B1', 'Non-defining relative clauses', 'The film, ___ won three Oscars, is now available on streaming.', '["who","that","whose","which"]', 'which'),

-- Gerunds and Infinitives
('B1', 'Gerunds and Infinitives', 'She admitted ___ the window.', '["to break","breaking","broke","break"]', 'breaking'),
('B1', 'Gerunds and Infinitives', 'I hope ___ you again soon.', '["seeing","to see","see","saw"]', 'to see'),
('B1', 'Gerunds and Infinitives', 'They suggested ___ a new approach.', '["to try","trying","tried","try"]', 'trying'),
('B1', 'Gerunds and Infinitives', 'He refused ___ the contract.', '["signing","to sign","sign","signed"]', 'to sign'),

-- Second conditional
('B1', 'Second conditional', 'If I ___ rich, I would buy a big house.', '["am","will be","had been","were"]', 'were'),
('B1', 'Second conditional', 'She would travel more if she ___ more money.', '["has","have","would have","had"]', 'had'),
('B1', 'Second conditional', 'If he ___ harder, he would get promoted.', '["works","work","will work","worked"]', 'worked'),
('B1', 'Second conditional', 'What would you do if you ___ the lottery?', '["win","wins","would win","won"]', 'won'),
('B1', 'Second conditional', 'I ___ you if I knew the answer.', '["tell","will tell","told","would tell"]', 'would tell'),
('B1', 'Second conditional', 'If they ___ here, they would help us.', '["are","will be","had been","were"]', 'were'),
('B1', 'Second conditional', 'She would be happier if she ___ a different job.', '["has","have","would have","had"]', 'had'),
('B1', 'Second conditional', 'If it ___ warmer, we could have a picnic.', '["is","will be","had been","were"]', 'were'),
('B1', 'Second conditional', 'She would feel better if she ___ more exercise.', '["does","would do","do","did"]', 'did'),
('B1', 'Second conditional', 'If we ___ a bigger flat, we''d invite more people.', '["have","would have","has","had"]', 'had'),

-- Third conditional
('B1', 'Third conditional', 'If she had studied harder, she ___ the exam.', '["would pass","had passed","will have passed","would have passed"]', 'would have passed'),
('B1', 'Third conditional', 'He ___ the job if he had arrived on time.', '["would get","had got","will have got","would have got"]', 'would have got'),
('B1', 'Third conditional', 'I wouldn''t have been late if the bus ___ on time.', '["came","comes","would come","had come"]', 'had come'),
('B1', 'Third conditional', 'If they had listened to the advice, they ___ that mistake.', '["wouldn''t make","hadn''t made","won''t make","wouldn''t have made"]', 'wouldn''t have made'),
('B1', 'Third conditional', 'She ___ so tired if she had slept more.', '["wouldn''t be","hadn''t been","won''t be","wouldn''t have been"]', 'wouldn''t have been'),
('B1', 'Third conditional', 'If you had told me, I ___ you.', '["would help","had helped","will have helped","would have helped"]', 'would have helped'),
('B1', 'Third conditional', 'He would have become a doctor if he ___ medicine.', '["studied","would study","studies","had studied"]', 'had studied'),
('B1', 'Third conditional', 'If I ___ about the party, I would have come.', '["knew","would know","know","had known"]', 'had known'),
('B1', 'Third conditional', 'They ___ the game if they had trained harder.', '["would win","had won","will have won","would have won"]', 'would have won'),
('B1', 'Third conditional', 'If she ___ the map, she wouldn''t have got lost.', '["checked","would check","checks","had checked"]', 'had checked'),

-- must / have to / needn't
('B1', 'must / have to / needn''t', 'You ___ wear a seatbelt — it''s the law.', '["needn''t","should","could","must"]', 'must'),
('B1', 'must / have to / needn''t', 'She ___ finish the report today — it''s not urgent.', '["mustn''t","needn''t","can''t","doesn''t have to"]', 'doesn''t have to'),
('B1', 'must / have to / needn''t', 'I ___ wake up early — I have a flight at 6 am.', '["needn''t","should","might","have to"]', 'have to'),
('B1', 'must / have to / needn''t', 'You ___ touch that — it''s very hot!', '["needn''t","don''t have to","shouldn''t","mustn''t"]', 'mustn''t'),
('B1', 'must / have to / needn''t', 'He ___ bring anything — we have plenty of food.', '["mustn''t","have to","must","needn''t"]', 'needn''t'),
('B1', 'must / have to / needn''t', 'Students ___ wear a uniform at this school.', '["needn''t","should","might","must"]', 'must'),
('B1', 'must / have to / needn''t', 'I ___ go — it''s getting late.', '["needn''t","might","would","must"]', 'must'),
('B1', 'must / have to / needn''t', 'You ___ be quiet in the library.', '["needn''t","could","would","must"]', 'must'),
('B1', 'must / have to / needn''t', 'You ___ buy a ticket — the event is free.', '["mustn''t","must","should","don''t have to"]', 'don''t have to'),
('B1', 'must / have to / needn''t', 'Visitors ___ use their phones during the tour.', '["don''t have to","needn''t","couldn''t","mustn''t"]', 'mustn''t'),

-- should have / could have / might have
('B1', 'should have / could have / might have', 'You ___ called me. I was waiting!', '["could have","might have","would have","should have"]', 'should have'),
('B1', 'should have / could have / might have', 'She ___ taken the earlier train — then she wouldn''t be late.', '["could have","must have","would have","should have"]', 'should have'),
('B1', 'should have / could have / might have', 'He ___ known about it — it was in the news.', '["should have","could have","must have","might have"]', 'might have'),
('B1', 'should have / could have / might have', 'They ___ left without saying goodbye.', '["couldn''t have","mightn''t have","mustn''t have","shouldn''t have"]', 'shouldn''t have'),
('B1', 'should have / could have / might have', 'I ___ locked the door — I can''t remember.', '["should have","could have","must have","might have"]', 'might have'),
('B1', 'should have / could have / might have', 'She looks exhausted. She ___ worked all night.', '["should have","could have","might have","must have"]', 'must have'),
('B1', 'should have / could have / might have', 'You ___ told me you were coming — I would have prepared something.', '["should have","might have","must have","could have"]', 'could have'),
('B1', 'should have / could have / might have', 'He ___ been at the party — I saw his car outside.', '["should have","could have","might have","must have"]', 'must have'),
('B1', 'should have / could have / might have', 'I ___ studied harder for the exam.', '["could have","might have","must have","should have"]', 'should have'),
('B1', 'should have / could have / might have', 'She ___ forgotten — she sometimes does.', '["must have","should have","could have","might have"]', 'might have'),

-- used to / be used to / get used to
('B1', 'used to / be used to / get used to', 'I ___ live in Paris when I was a child.', '["am used to","get used to","got used to","used to"]', 'used to'),
('B1', 'used to / be used to / get used to', 'She ___ waking up early now — it''s no problem for her.', '["used to","gets used to","got used","is used to"]', 'is used to'),
('B1', 'used to / be used to / get used to', 'He is ___ the cold weather — he''s from Canada.', '["use to","getting used","uses to","used to"]', 'used to'),
('B1', 'used to / be used to / get used to', 'They ___ have a dog, but it died last year.', '["are used to","get used to","got used","used to"]', 'used to'),
('B1', 'used to / be used to / get used to', 'It took me a while, but I ___ driving on the left.', '["used to","am used to","use to","got used to"]', 'got used to'),
('B1', 'used to / be used to / get used to', 'She ___ never eat vegetables, but now she loves them.', '["is used to","gets used to","has used to","used to"]', 'used to'),
('B1', 'used to / be used to / get used to', 'Are you ___ the new schedule yet?', '["used to","get used to","use to","getting used to"]', 'getting used to'),
('B1', 'used to / be used to / get used to', 'He ___ smoke 20 cigarettes a day.', '["is used to","gets used to","got used","used to"]', 'used to'),
('B1', 'used to / be used to / get used to', 'I am slowly ___ living alone.', '["used to","get used","uses to","getting used to"]', 'getting used to'),
('B1', 'used to / be used to / get used to', 'She ___ the noise — she lives near a busy road.', '["used to","gets used to","got used","is used to"]', 'is used to'),
('B1', 'used to / be used to / get used to', 'Did you ___ walk to school when you were young?', '["used to","get used to","are used to","use to"]', 'use to'),

-- ============================================================
-- B2 — Upper-Intermediate
-- ============================================================

-- Wish / If only
('B2', 'Wish/If only', 'I wish I ___ taller.', '["am","was","were","be"]', 'were'),
('B2', 'Wish/If only', 'She wishes she ___ more time to finish the project.', '["has","have","had","has had"]', 'had'),
('B2', 'Wish/If only', 'I wish I ___ harder for the exam.', '["studied","had studied","have studied","study"]', 'had studied'),
('B2', 'Wish/If only', 'If only he ___ here with us now!', '["is","was","were","will be"]', 'were'),

-- Cleft Sentences
('B2', 'Cleft Sentences', '___ I really need is a good rest.', '["That","What","Which","Who"]', 'What'),
('B2', 'Cleft Sentences', 'It was last year ___ they got married.', '["when","that","which","where"]', 'that'),
('B2', 'Cleft Sentences', 'It is the economy ___ worries people most.', '["which","who","that","whose"]', 'that'),

-- Causative have / get
('B2', 'Causative', 'The manager made his staff ___ overtime.', '["work","to work","working","worked"]', 'work'),
('B2', 'Causative', 'She had her assistant ___ all the reports.', '["type","to type","typing","typed"]', 'type'),
('B2', 'Causative', 'They got the plumber ___ the leak.', '["fix","to fix","fixing","fixed"]', 'to fix'),

-- Participle Clauses
('B2', 'Participle Clauses', 'Having ___ the contract, he felt relieved.', '["sign","signing","signed","to sign"]', 'signed'),
('B2', 'Participle Clauses', '___ in 1887, the building is now a museum.', '["Building","Built","Having built","Being built"]', 'Built'),
('B2', 'Participle Clauses', 'The woman ___ at the reception is my manager.', '["sat","sitting","who sitting","sits"]', 'sitting'),

-- Concession
('B2', 'Concession', '___ the bad weather, we enjoyed the trip.', '["Although","Despite","However","Even though"]', 'Despite'),
('B2', 'Concession', 'He passed the exam ___ having little time to study.', '["although","in spite of","despite of","however"]', 'in spite of'),
('B2', 'Concession', '___ she tried very hard, she didn''t succeed.', '["Despite","However","Although","In spite of"]', 'Although'),

-- ============================================================
-- C1 — Advanced
-- ============================================================

-- Advanced Modals
('C1', 'Advanced Modals', 'By now, she ___ arrived — her flight was at noon.', '["must have","should have","would have","might"]', 'must have'),
('C1', 'Advanced Modals', 'Don''t worry, she ___ not have received your message yet.', '["must","can","may","should"]', 'may'),
('C1', 'Advanced Modals', 'He ___ have been lying — all the evidence pointed to him.', '["might","must","could","would"]', 'must'),
('C1', 'Advanced Modals', 'I ___ have helped you move, but I was away that weekend.', '["could","would","should","might"]', 'could'),

-- Ellipsis
('C1', 'Ellipsis', 'She speaks better than I ___.', '["do","am","does","be"]', 'do'),
('C1', 'Ellipsis', 'I''ve finished my homework and so ___ Tom.', '["has","have","did","does"]', 'has'),
('C1', 'Ellipsis', 'You could have asked me, and I would ___.', '["do","have","did","be"]', 'have'),

-- Emphasis
('C1', 'Emphasis', '___ that I find most difficult is the pronunciation.', '["That","What","Which","It"]', 'What'),
('C1', 'Emphasis', 'Rarely ___ I seen such dedication.', '["did","have","was","had"]', 'have'),
('C1', 'Emphasis', 'Only after the meeting ___ he realise his mistake.', '["he did","did he","had he","he had"]', 'did he'),

-- Hypothetical structures
('C1', 'Hypothetical structures', 'It''s time we ___ a decision.', '["make","made","making","have made"]', 'made'),
('C1', 'Hypothetical structures', 'I''d sooner you ___ him about this.', '["don''t tell","didn''t tell","wouldn''t tell","haven''t told"]', 'didn''t tell'),
('C1', 'Hypothetical structures', 'Suppose you ___ wrong — what would you do?', '["are","were","be","will be"]', 'were'),

-- ============================================================
-- C2 — Proficiency
-- ============================================================

-- Nominalization
('C2', 'Nominalization', 'The ___ of the new law caused controversy.', '["introduce","introducing","introduction","introduced"]', 'introduction'),
('C2', 'Nominalization', 'We were surprised by his ___ to help.', '["willing","willingness","willingly","willed"]', 'willingness'),
('C2', 'Nominalization', 'The ___ of refugees has become a global issue.', '["displace","displacement","displacing","displaced"]', 'displacement'),

-- Absolute Constructions
('C2', 'Absolute Constructions', 'The meeting ___, everyone left the building.', '["over","is over","being over","having over"]', 'over'),
('C2', 'Absolute Constructions', '___ all options exhausted, the team turned to outside help.', '["With","Having","After","All"]', 'With'),
('C2', 'Absolute Constructions', 'The sun ___ set, the temperature dropped quickly.', '["having","being","to have","has"]', 'having'),

-- Complex Conditionals
('C2', 'Complex Conditionals', '___ you to reconsider, we would welcome your return.', '["Should","Would","Were","Had"]', 'Should'),
('C2', 'Complex Conditionals', '___ sooner had they signed the deal than problems emerged.', '["Not","No","Never","Scarcely"]', 'No'),
('C2', 'Complex Conditionals', 'Had he ___ the warning, the accident could have been avoided.', '["heed","heeded","heeding","to heed"]', 'heeded'),

-- Formal Register
('C2', 'Formal Register', 'The study ___ to examine the relationship between stress and performance.', '["seeks","looked","is seeking","sought"]', 'sought'),
('C2', 'Formal Register', 'The data ___ to a significant increase in unemployment.', '["points","point","are pointing","have pointed"]', 'point'),
('C2', 'Formal Register', 'It ___ argued that the policy has consistently failed.', '["can","has been","could","might been"]', 'has been');

-- +goose Down
DELETE FROM grammar_questions WHERE category IN (
    'am/is/are',
    'Present Simple',
    'Present Continuous',
    'Present Simple vs Present Continuous',
    'there is / there are',
    'Have/Have got',
    'Some/Any',
    'Articles: a/an',
    'Prepositions of time',
    'Prepositions of place',
    'Question words',
    'Adjectives and adverbs',
    'Past Simple (irregular)',
    'Past Continuous',
    'Present Perfect',
    'Going to',
    'will (future)',
    'Comparatives',
    'Superlatives',
    'First conditional',
    'should / ought to',
    'must/have to/don''t have to',
    'would like',
    'Articles: the',
    'too / enough',
    'Past Perfect',
    'Present Perfect Continuous',
    'Passive Voice',
    'Reported Speech',
    'Relative Clauses',
    'Non-defining relative clauses',
    'Gerunds and Infinitives',
    'Second conditional',
    'Third conditional',
    'must / have to / needn''t',
    'should have / could have / might have',
    'used to / be used to / get used to',
    'Wish/If only',
    'Cleft Sentences',
    'Causative',
    'Participle Clauses',
    'Concession',
    'Advanced Modals',
    'Ellipsis',
    'Emphasis',
    'Hypothetical structures',
    'Nominalization',
    'Absolute Constructions',
    'Complex Conditionals',
    'Formal Register'
);
