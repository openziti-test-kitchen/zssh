# zssh
Ziti SSH is a project to replace `ssh` with a more secure, zero-trust implementation of `ssh`.

I can tell you're skeptical. I can tell you're wondering why in the world we would
even attempt to mess with `ssh` at all. After all, `ssh` has been a foundation of the administration of not only home
networks but also corporate networks and the internet itself. Surely if millions (billions?) of computers can interact
every day safely and securely using `ssh` there is "no need" for us to be spending time zitifying `ssh`
right? (Spoiler alert: wrong)

I'm sure you've guessed that this is not the case whatsoever. After all, attackers don't leave `ssh` alone just because
it's not worth it to try! Put a machine on the open internet, expose `ssh` on port 22 and watch for yourself all the
attempts to access `ssh` using known default/weak/bad passwords flood in. Attacks don't only come from the internet
either! Attacks from a single compromised machine on your network very well could behave in the same way as an outside
attacker. This is particularly true for ransomware-style attacks as the compromised machine attempts to expand/multiply.
The problems don't just stop here either. DoS attacks, other zero-day type bugs and more are all waiting for any service
sitting on the open internet.

A zitified `ssh` client is superior since the port used by `ssh` can be eliminated from the internet-based firewall
preventing any connections whatsoever from any network client. In this configuration the `ssh` process is effectively "
dark". The only way to
`ssh` to a machine configured in this way is to have an identity authorized for
that [Ziti Network](https://openziti.github.io/ziti/overview.html#overview-of-a-ziti-network). Cool right? Let's see how
we did it and how you can do the same thing using
a [Ziti Network](https://openziti.github.io/ziti/overview.html#overview-of-a-ziti-network).
