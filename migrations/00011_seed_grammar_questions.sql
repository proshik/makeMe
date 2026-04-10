-- +goose Up
INSERT INTO grammar_questions (level, category, prompt, options, answer) VALUES
-- A1: Present Simple / to be / to have
('A1', 'Present Simple', 'She ___ a teacher.', '["is","are","am","be"]', 'is'),
('A1', 'Present Simple', 'We ___ students.', '["is","am","are","be"]', 'are'),
('A1', 'Present Simple / have', 'He ___ a car.', '["have","has","having","haves"]', 'has'),
('A1', 'Present Simple', 'I ___ English every day.', '["study","studies","studying","studied"]', 'study'),
('A1', 'Present Simple', 'The cat ___ on the table.', '["sit","sits","sitting","sitted"]', 'sits'),

-- A2: Past Simple / Present Continuous / Comparatives
('A2', 'Past Simple', 'She ___ to the store yesterday.', '["go","goes","went","gone"]', 'went'),
('A2', 'Present Continuous', 'I ___ my homework right now.', '["do","does","am doing","did"]', 'am doing'),
('A2', 'Present Perfect', 'They ___ already ___ lunch.', '["have...eaten","has...eaten","have...ate","had...eat"]', 'have...eaten'),
('A2', 'Comparatives', 'The movie was ___ than the book.', '["good","better","best","more good"]', 'better'),
('A2', 'Modal Verbs', 'You ___ turn off the lights when you leave.', '["should","shall","would","could"]', 'should'),

-- B1: Conditionals / Passive / Reported Speech
('B1', 'Conditionals', 'If I ___ rich, I would travel.', '["am","was","were","be"]', 'were'),
('B1', 'Passive Voice', 'The letter ___ yesterday.', '["sent","was sent","is sent","sends"]', 'was sent'),
('B1', 'Reported Speech', 'She told me that she ___ busy.', '["is","was","were","been"]', 'was'),
('B1', 'Present Perfect', 'I''ve been working here ___ five years.', '["since","for","from","during"]', 'for'),
('B1', 'Modal Verbs', 'He ___ have arrived by now.', '["must","can","shall","would"]', 'must'),

-- B2: Advanced Conditionals / Future Perfect Passive / Inversion
('B2', 'Conditionals', 'Had she studied harder, she ___ the exam.', '["passed","would pass","would have passed","had passed"]', 'would have passed'),
('B2', 'Future Perfect Passive', 'The report ___ by the time you arrive.', '["will finish","will be finished","will have been finished","finishes"]', 'will have been finished'),
('B2', 'Inversion', 'Not only ___ he apologize, but he also paid.', '["does","did","had","was"]', 'did'),
('B2', 'Gerunds and Infinitives', 'I regret ___ you earlier.', '["not telling","not to tell","to not tell","not tell"]', 'not telling'),
('B2', 'Subjunctive', 'It''s high time we ___ a decision.', '["make","made","making","have made"]', 'made'),

-- C1: Inversion / Stative Verbs / Conditional Inversion
('C1', 'Inversion', 'Little ___ he know what awaited him.', '["does","did","had","was"]', 'did'),
('C1', 'Stative Verbs', 'The evidence ___ to suggest a different conclusion.', '["seems","is seeming","seem","has seeming"]', 'seems'),
('C1', 'Conditional Inversion', 'Were he ___ the truth, things would be different.', '["to tell","telling","told","tell"]', 'to tell'),
('C1', 'Purpose Clauses', 'She left early ___ miss the traffic.', '["so as not to","for not to","so that not","in order not"]', 'so as not to'),
('C1', 'Inversion', 'Seldom ___ such a performance.', '["have I seen","I have seen","I saw","did I saw"]', 'have I seen'),

-- C2: Fixed expressions / Inversion / Subjunctive
('C2', 'Fixed Expressions', 'Be that ___ it may, we must proceed.', '["as","so","what","how"]', 'as'),
('C2', 'Inversion', 'So pervasive ___ the influence that none escaped it.', '["was","is","were","had"]', 'was'),
('C2', 'Comparison Clauses', 'He spoke ___ he had witnessed the event firsthand.', '["as though","like if","as like","so that"]', 'as though'),
('C2', 'Subject-Verb Agreement', 'The committee ___ yet to reach a consensus.', '["has","have","had","is"]', 'has'),
('C2', 'Concession', 'Try ___ he might, success eluded him.', '["as","though","hard","much"]', 'as');

-- +goose Down
DELETE FROM grammar_questions;
