package HelloWorld;
sub new
{
    my $class = shift;
    my $self = { };
    bless $self, $class;
    return $self;
}
sub print
{
	eval {
	    print "Hello World!\n";
	}
}

package main;
$hw = HelloWorld->new();
while (1) {
	$hw->print();
}
