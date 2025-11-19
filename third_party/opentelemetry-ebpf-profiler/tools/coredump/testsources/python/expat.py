import xml.parsers.expat

def test():
    while True:
        pass

# 3 handler functions
def start_element(name, attrs):
    print('Start element:', name, attrs)
    test()
def end_element(name):
    print('End element:', name)
def char_data(data):
    print('Character data:', repr(data))

def main():
    p = xml.parsers.expat.ParserCreate()

    p.StartElementHandler = start_element
    p.EndElementHandler = end_element
    p.CharacterDataHandler = char_data

    p.Parse("""<?xml version="1.0"?>
<parent id="top"><child1 name="paul">Text goes here</child1>
<child2 name="fred">More text</child2>
</parent>""", 1)

main()
