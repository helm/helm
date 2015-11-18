#!/bin/bash
set -e

if [ "$RABBITMQ_ERLANG_COOKIE" ]; then
	cookieFile='/var/lib/rabbitmq/.erlang.cookie'
	if [ -e "$cookieFile" ]; then
		if [ "$(cat "$cookieFile" 2>/dev/null)" != "$RABBITMQ_ERLANG_COOKIE" ]; then
			echo >&2
			echo >&2 "warning: $cookieFile contents do not match RABBITMQ_ERLANG_COOKIE"
			echo >&2
		fi
	else
		echo "$RABBITMQ_ERLANG_COOKIE" > "$cookieFile"
		chmod 600 "$cookieFile"
		chown rabbitmq "$cookieFile"
	fi
fi

if [ "$1" = 'rabbitmq-server' ]; then
	configs=(
		# https://www.rabbitmq.com/configure.html
		default_vhost
		default_user
		default_pass
	)

	haveConfig=
	for conf in "${configs[@]}"; do
		var="RABBITMQ_${conf^^}"
		val="${!var}"
		if [ "$val" ]; then
			haveConfig=1
			break
		fi
	done

	if [ "$haveConfig" ]; then
		cat > /etc/rabbitmq/rabbitmq.config <<-'EOH'
			[
			  {rabbit,
			    [
		EOH
		for conf in "${configs[@]}"; do
			var="RABBITMQ_${conf^^}"
			val="${!var}"
			[ "$val" ] || continue
			cat >> /etc/rabbitmq/rabbitmq.config <<-EOC
			      {$conf, <<"$val">>},
			EOC
		done
		cat >> /etc/rabbitmq/rabbitmq.config <<-'EOF'
			      {loopback_users, []}
			    ]
			  }
			].
		EOF
	fi

	chown -R rabbitmq /var/lib/rabbitmq
	set -- gosu rabbitmq "$@"
fi

mkdir -p /var/lib/rabbitmq/mnesia
cd /var/lib/rabbitmq/mnesia
if [ -e rabbit@hostname ];
then
  OLDHOSTNAME=$(readlink rabbit@hostname)
  # Copy old database over to current hostname
  # cp rabbit@hostname.pid rabbit@`hostname`.pid || :
  cd rabbit@hostname
  # Replace any references to the old hostname to the current one
  # Add this behind the above command to remove everything in front of the @
  # symbol (including the @) | grep -o '@.*' | cut -f2- -d'@'
  find . -type f -exec sed -i "s/$OLDHOSTNAME/rabbit@$(hostname)/g" {} +
  cd ..
  cp -r "$OLDHOSTNAME" rabbit@`hostname`
  cp -r "$OLDHOSTNAME-plugins-expand" rabbit@`hostname`-plugins-expand
  rm -fr "$OLDHOSTNAME"
  rm -fr "$OLDHOSTNAME-plugins-expand"
fi
# Link current database to the rabbit@hostname files
ln -sfn rabbit@`hostname` rabbit@hostname
# ln -sfn rabbit@`hostname`.pid rabbit@hostname.pid
ln -sfn rabbit@`hostname`-plugins-expand rabbit@hostname-plugins-expand
chown -R rabbitmq:rabbitmq /var/lib/rabbitmq

exec "$@"
